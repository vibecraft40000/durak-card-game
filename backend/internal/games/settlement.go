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

// PayoutEntry is one player's payout (positive = win, negative = loss).
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

func computePayouts(state engine.GameState, stake float64, commissionBps int) (pot float64, commission float64, payouts []PayoutEntry) {
	pot = stake * float64(len(state.PlayerOrder))
	commission = pot * float64(commissionBps) / 10000
	winAmount := pot - commission
	// 2 players: winner gets winAmount, loser loses stake
	// winner profit = winAmount - stake, loser = -stake
	for _, playerID := range state.PlayerOrder {
		if playerID == state.WinnerPlayer {
			payouts = append(payouts, PayoutEntry{UserID: playerID, Amount: winAmount - stake})
		} else {
			payouts = append(payouts, PayoutEntry{UserID: playerID, Amount: -stake})
		}
	}
	return pot, commission, payouts
}

// SettleIfFinished runs settlement and returns PayoutInfo for match_finished. Uses SETNX to prevent double payout.
// Wallet updates and game_results insert run in a single DB transaction for atomicity.
func SettleIfFinished(ctx context.Context, db *pgxpool.Pool, redisClient *redis.Client, walletService *wallet.Service, gameResultsRepo *gameresults.Repository, state engine.GameState, stake float64, commissionBps int) (*PayoutInfo, error) {
	if state.Status != engine.StatusFinished || state.WinnerPlayer == "" {
		return nil, nil
	}
	pot, commission, payouts := computePayouts(state, stake, commissionBps)
	settlementID := uuid.NewString()
	key := "payout:" + state.MatchID

	ok, err := redisClient.SetNX(ctx, key, "1", payoutKeyTTL).Result()
	if err != nil {
		return nil, err
	}
	if ok {
		if db == nil {
			if err := walletService.SettleWin(ctx, state.WinnerPlayer, state.MatchID, pot, commissionBps); err != nil {
				redisClient.Del(ctx, key)
				return nil, err
			}
			entries := make([]gameresults.Entry, 0, len(state.PlayerOrder))
			winAmount := pot - commission
			for _, playerID := range state.PlayerOrder {
				if playerID == state.WinnerPlayer {
					entries = append(entries, gameresults.Entry{
						MatchID: state.MatchID, UserID: playerID, Stake: stake,
						Payout: winAmount, Profit: winAmount - stake,
					})
				} else {
					entries = append(entries, gameresults.Entry{
						MatchID: state.MatchID, UserID: playerID, Stake: stake,
						Payout: 0, Profit: -stake,
					})
				}
			}
			if gameResultsRepo != nil && len(entries) > 0 {
				_ = gameResultsRepo.Insert(ctx, entries)
			}
		} else {
			tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
			if err != nil {
				redisClient.Del(ctx, key)
				return nil, err
			}
			defer tx.Rollback(ctx)
			if err := walletService.SettleWinInTx(ctx, tx, state.WinnerPlayer, state.MatchID, pot, commissionBps); err != nil {
				redisClient.Del(ctx, key)
				return nil, err
			}
			entries := make([]gameresults.Entry, 0, len(state.PlayerOrder))
			winAmount := pot - commission
			for _, playerID := range state.PlayerOrder {
				if playerID == state.WinnerPlayer {
					entries = append(entries, gameresults.Entry{
						MatchID: state.MatchID, UserID: playerID, Stake: stake,
						Payout: winAmount, Profit: winAmount - stake,
					})
				} else {
					entries = append(entries, gameresults.Entry{
						MatchID: state.MatchID, UserID: playerID, Stake: stake,
						Payout: 0, Profit: -stake,
					})
				}
			}
			if gameResultsRepo != nil && len(entries) > 0 {
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
		info := &PayoutInfo{SettlementID: settlementID, Payouts: payouts, Commission: commission, Pot: pot}
		info.NewBalances = make(map[string]float64)
		for _, playerID := range state.PlayerOrder {
			if b, err := walletService.Balance(ctx, playerID); err == nil {
				info.NewBalances[playerID] = b
			}
		}
		raw, _ := json.Marshal(info)
		redisClient.Set(ctx, "payout:result:"+state.MatchID, raw, payoutKeyTTL)
		return info, nil
	}

	// Already settled by another goroutine/pod; try to get stored result
	raw, err := redisClient.Get(ctx, "payout:result:"+state.MatchID).Bytes()
	if err == nil {
		var info PayoutInfo
		if json.Unmarshal(raw, &info) == nil {
			return &info, nil
		}
	}
	// Fallback: return computed payouts without settlementId
	return &PayoutInfo{Payouts: payouts, Commission: commission, Pot: pot}, nil
}
