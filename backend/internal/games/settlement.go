package games

import (
	"context"
	"encoding/json"
	"time"

	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/gameresults"
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

func placePercents(playersCount int, commissionBps int) []float64 {
	commissionPercent := float64(commissionBps) / 100.0
	netPercent := 100.0 - commissionPercent
	if netPercent < 0 {
		netPercent = 0
	}

	switch playersCount {
	case 2:
		return []float64{netPercent}
	case 3:
		return []float64{netPercent * 0.6, netPercent * 0.4}
	case 4:
		return []float64{netPercent * 0.5, netPercent * 0.3, netPercent * 0.2}
	default:
		return []float64{netPercent}
	}
}

func computePayouts(state engine.GameState, stake float64, commissionBps int) (pot float64, commission float64, payouts []PayoutEntry, grossByUser map[string]float64) {
	pot = stake * float64(len(state.PlayerOrder))
	commission = pot * float64(commissionBps) / 10000

	groups := normalizedFinishGroups(state)
	percents := placePercents(len(state.PlayerOrder), commissionBps)
	grossByUser = make(map[string]float64, len(state.PlayerOrder))

	place := 1
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		sumPercent := 0.0
		for i := 0; i < len(group); i++ {
			placeIndex := place + i - 1
			if placeIndex < 0 || placeIndex >= len(percents) {
				continue
			}
			sumPercent += percents[placeIndex]
		}
		sharePercent := sumPercent / float64(len(group))
		gross := pot * sharePercent / 100.0
		for _, playerID := range group {
			grossByUser[playerID] = gross
		}
		place += len(group)
	}

	payouts = make([]PayoutEntry, 0, len(state.PlayerOrder))
	for _, playerID := range state.PlayerOrder {
		gross := grossByUser[playerID]
		profit := gross - stake
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
