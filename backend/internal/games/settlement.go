package games

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"

	"durakonline/backend/internal/gameresults"
	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/wallet"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// PayoutEntry is one player's net result (profit/loss) for UI.
type PayoutEntry struct {
	UserID string  `json:"userId"`
	Amount float64 `json:"amount"`
}

// PayoutInfo is sent to clients in match_finished.
type PayoutInfo struct {
	SettlementID string             `json:"settlementId"`
	Payouts      []PayoutEntry      `json:"payouts"`
	Commission   float64            `json:"commission"`
	Pot          float64            `json:"pot"`
	NewBalances  map[string]float64 `json:"newBalances,omitempty"`
}

const payoutKeyTTL = 30 * time.Minute

func normalizedFinishGroups(state engine.GameState) [][]string {
	if len(state.FinishGroups) > 0 {
		return state.FinishGroups
	}
	if state.WinnerPlayer == "" {
		return nil
	}
	groups := [][]string{{state.WinnerPlayer}}
	for _, playerID := range state.PlayerOrder {
		if playerID == state.WinnerPlayer {
			continue
		}
		groups = append(groups, []string{playerID})
	}
	return groups
}

func placeWeights(playersCount int) []int64 {
	switch playersCount {
	case 2:
		return []int64{1000}
	case 3:
		return []int64{600, 400}
	case 4:
		return []int64{500, 300, 200}
	default:
		return []int64{1000}
	}
}

func cents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

func dollars(amountCents int64) float64 {
	return float64(amountCents) / 100.0
}

func roundDiv(numerator, denominator int64) int64 {
	if denominator == 0 {
		return 0
	}
	quotient := numerator / denominator
	remainder := numerator % denominator
	if remainder*2 >= denominator {
		quotient++
	}
	return quotient
}

func allocateByWeights(total int64, weights []int64) []int64 {
	if len(weights) == 0 || total <= 0 {
		return make([]int64, len(weights))
	}
	var weightSum int64
	for _, weight := range weights {
		weightSum += weight
	}
	if weightSum <= 0 {
		return make([]int64, len(weights))
	}

	type weightedRemainder struct {
		index     int
		remainder int64
	}

	allocations := make([]int64, len(weights))
	remainders := make([]weightedRemainder, len(weights))
	var distributed int64
	for i, weight := range weights {
		numerator := total * weight
		allocations[i] = numerator / weightSum
		distributed += allocations[i]
		remainders[i] = weightedRemainder{
			index:     i,
			remainder: numerator % weightSum,
		}
	}

	sort.SliceStable(remainders, func(i, j int) bool {
		if remainders[i].remainder == remainders[j].remainder {
			return remainders[i].index < remainders[j].index
		}
		return remainders[i].remainder > remainders[j].remainder
	})

	for remaining := total - distributed; remaining > 0; remaining-- {
		allocations[remainders[0].index]++
		remainders = append(remainders[1:], remainders[0])
	}

	return allocations
}

func orderedTieGroup(group []string, playerOrder []string) []string {
	ordered := make([]string, 0, len(group))
	groupSet := make(map[string]struct{}, len(group))
	for _, playerID := range group {
		groupSet[playerID] = struct{}{}
	}
	for _, playerID := range playerOrder {
		if _, ok := groupSet[playerID]; ok {
			ordered = append(ordered, playerID)
			delete(groupSet, playerID)
		}
	}
	for _, playerID := range group {
		if _, ok := groupSet[playerID]; ok {
			ordered = append(ordered, playerID)
			delete(groupSet, playerID)
		}
	}
	return ordered
}

func computePayouts(state engine.GameState, stake float64, commissionBps int) (pot float64, commission float64, payouts []PayoutEntry, grossByUser map[string]float64) {
	stakeCents := cents(stake)
	potCents := stakeCents * int64(len(state.PlayerOrder))
	commissionCents := roundDiv(potCents*int64(commissionBps), 10000)
	distributableCents := potCents - commissionCents
	if distributableCents < 0 {
		distributableCents = 0
	}

	pot = dollars(potCents)
	commission = dollars(commissionCents)

	groups := normalizedFinishGroups(state)
	placeAllocations := allocateByWeights(distributableCents, placeWeights(len(state.PlayerOrder)))
	grossByUser = make(map[string]float64, len(state.PlayerOrder))
	grossByUserCents := make(map[string]int64, len(state.PlayerOrder))

	place := 1
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		group = orderedTieGroup(group, state.PlayerOrder)
		var groupTotal int64
		for i := 0; i < len(group); i++ {
			placeIndex := place + i - 1
			if placeIndex < 0 || placeIndex >= len(placeAllocations) {
				continue
			}
			groupTotal += placeAllocations[placeIndex]
		}
		if groupTotal <= 0 {
			place += len(group)
			continue
		}
		shareBase := groupTotal / int64(len(group))
		shareRemainder := groupTotal % int64(len(group))
		for i, playerID := range group {
			grossCents := shareBase
			if int64(i) < shareRemainder {
				grossCents++
			}
			grossByUserCents[playerID] = grossCents
			grossByUser[playerID] = dollars(grossCents)
		}
		place += len(group)
	}

	payouts = make([]PayoutEntry, 0, len(state.PlayerOrder))
	for _, playerID := range state.PlayerOrder {
		grossCents := grossByUserCents[playerID]
		profit := dollars(grossCents - stakeCents)
		payouts = append(payouts, PayoutEntry{
			UserID: playerID,
			Amount: profit,
		})
	}
	return pot, commission, payouts, grossByUser
}

// SettleIfFinished runs settlement and returns PayoutInfo for match_finished.
// Uses SETNX to prevent double payout.
func SettleIfFinished(ctx context.Context, db *pgxpool.Pool, redisClient *redis.Client, walletService *wallet.Service, gameResultsRepo *gameresults.Repository, state engine.GameState, stake float64, commissionBps int) (*PayoutInfo, error) {
	if state.Status != engine.StatusFinished {
		return nil, nil
	}
	pot, commission, payouts, grossByUser := computePayouts(state, stake, commissionBps)
	settlementID := uuid.NewString()
	key := "payout:" + state.MatchID

	ok, err := redisClient.SetNX(ctx, key, "1", payoutKeyTTL).Result()
	if err != nil {
		return nil, err
	}
	if ok {
		if db != nil {
			tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
			if err != nil {
				redisClient.Del(ctx, key)
				return nil, err
			}
			defer tx.Rollback(ctx)

			for _, playerID := range state.PlayerOrder {
				gross := grossByUser[playerID]
				if gross <= 0 {
					continue
				}
				if _, err := tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES (gen_random_uuid(), $1, 'win', $2, 'confirmed', $3, NOW())
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING`,
					playerID, gross, state.MatchID); err != nil {
					redisClient.Del(ctx, key)
					return nil, err
				}
			}
			if _, err := tx.Exec(ctx, `
INSERT INTO platform_fees (id, match_id, gross_pot, commission_bps, commission_amount, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW())
ON CONFLICT (match_id) DO NOTHING`,
				state.MatchID, pot, commissionBps, commission); err != nil {
				redisClient.Del(ctx, key)
				return nil, err
			}

			if gameResultsRepo != nil {
				entries := make([]gameresults.Entry, 0, len(state.PlayerOrder))
				for _, playerID := range state.PlayerOrder {
					gross := grossByUser[playerID]
					entries = append(entries, gameresults.Entry{
						MatchID: state.MatchID,
						UserID:  playerID,
						Stake:   stake,
						Payout:  gross,
						Profit:  gross - stake,
					})
				}
				if err := gameResultsRepo.InsertWithTx(ctx, tx, entries); err != nil {
					redisClient.Del(ctx, key)
					return nil, err
				}
			}

			if err := tx.Commit(ctx); err != nil {
				redisClient.Del(ctx, key)
				return nil, err
			}
		}

		info := &PayoutInfo{
			SettlementID: settlementID,
			Payouts:      payouts,
			Commission:   commission,
			Pot:          pot,
		}
		if walletService != nil {
			info.NewBalances = make(map[string]float64)
			for _, playerID := range state.PlayerOrder {
				if b, err := walletService.Balance(ctx, playerID); err == nil {
					info.NewBalances[playerID] = b
				}
			}
		}
		raw, _ := json.Marshal(info)
		redisClient.Set(ctx, "payout:result:"+state.MatchID, raw, payoutKeyTTL)
		return info, nil
	}

	// Already settled by another goroutine/pod; try to get stored result.
	raw, err := redisClient.Get(ctx, "payout:result:"+state.MatchID).Bytes()
	if err == nil {
		var info PayoutInfo
		if json.Unmarshal(raw, &info) == nil {
			return &info, nil
		}
	}
	// Fallback: return computed payouts without settlementId.
	return &PayoutInfo{Payouts: payouts, Commission: commission, Pot: pot}, nil
}
