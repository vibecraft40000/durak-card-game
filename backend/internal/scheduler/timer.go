package scheduler

import (
	"context"
	"time"

	"durakonline/backend/internal/games"
	"durakonline/backend/internal/rooms"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/internal/ws"
	"durakonline/backend/pkg/metrics"
)

// disconnectCheckInterval: runs every 8s; 60s grace = ~7-8 checks before abandon.
const disconnectCheckInterval = 8 * time.Second

func RunDisconnectTimeouts(ctx context.Context, gamesService *games.Service, roomsService *rooms.Service, hub *ws.Hub, walletService *wallet.Service, commissionBps int, disableMoney bool) {
	ticker := time.NewTicker(disconnectCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			results := gamesService.HandleDisconnectTimeouts(ctx)
			activeRooms, _ := roomsService.List(ctx)
			for _, r := range results {
				var roomID string
				for _, room := range activeRooms {
					if room.MatchID == r.MatchID {
						roomID = room.ID
						break
					}
				}
				if roomID == "" {
					continue
				}
				room, err := roomsService.Get(ctx, roomID)
				if err != nil {
					continue
				}
				var payoutInfo *games.PayoutInfo
				if !disableMoney && !rooms.ContainsBotPlayerIn(r.State.PlayerOrder) {
					payoutInfo, _ = gamesService.SettleMatchIfFinished(ctx, walletService, r.State, room.Stake, commissionBps)
				}
				_, _ = roomsService.MarkRoomFinished(ctx, roomID)
				payload := map[string]any{
					"roomId":         roomID,
					"winnerPlayerId": r.State.WinnerPlayer,
					"abandoned":      true,
				}
				if payoutInfo != nil {
					payload["settlementId"] = payoutInfo.SettlementID
					payload["payouts"] = payoutInfo.Payouts
					payload["commission"] = payoutInfo.Commission
					payload["pot"] = payoutInfo.Pot
					if len(payoutInfo.NewBalances) > 0 {
						payload["newBalances"] = payoutInfo.NewBalances
					}
				}
				hub.Broadcast(roomID, ws.ServerEvent{Type: "match_finished", Payload: payload})
			}
		}
	}
}

// TimeoutBroadcaster broadcasts timeout-applied moves. Implemented by ws.Handler.
type TimeoutBroadcaster interface {
	BroadcastTimeoutApplied(ctx context.Context, roomID string, result *games.TimeoutResult, room rooms.Room)
}

func RunMatchTimers(ctx context.Context, gamesService *games.Service, roomsService *rooms.Service, hub *ws.Hub, broadcaster TimeoutBroadcaster) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	// Only broadcast timer_update when TurnEndsAt changed (avoids 50k broadcasts/sec at scale).
	lastTimerPayload := make(map[string]int64) // roomID -> last turnEndsAt unix ms

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			timeoutResults := gamesService.HandleTimeouts(ctx)
			for range timeoutResults {
				metrics.IncTimeoutApplied()
			}
			activeRooms, _ := roomsService.List(ctx)
			for _, result := range timeoutResults {
				var roomID string
				for _, room := range activeRooms {
					if room.MatchID == result.MatchID {
						roomID = room.ID
						break
					}
				}
				if roomID == "" || broadcaster == nil {
					continue
				}
				room, err := roomsService.Get(ctx, roomID)
				if err != nil {
					continue
				}
				broadcaster.BroadcastTimeoutApplied(ctx, roomID, &result, room)
			}
			for _, room := range activeRooms {
				if room.MatchID == "" {
					continue
				}
				state, err := gamesService.GetState(ctx, room.MatchID)
				if err != nil {
					continue
				}
				turnEndsMs := state.TurnEndsAt.UnixMilli()
				if last := lastTimerPayload[room.ID]; last == turnEndsMs {
					continue
				}
				lastTimerPayload[room.ID] = turnEndsMs
				hub.Broadcast(room.ID, ws.ServerEvent{
					Type: "timer_update",
					Payload: map[string]any{
						"roomId":       room.ID,
						"turnPlayerId": state.TurnPlayerID,
						"turnEndsAt":   turnEndsMs,
					},
				})
			}
			// Evict stale entries for rooms no longer in activeRooms
			activeByID := make(map[string]bool)
			for _, r := range activeRooms {
				if r.MatchID != "" {
					activeByID[r.ID] = true
				}
			}
			for rid := range lastTimerPayload {
				if !activeByID[rid] {
					delete(lastTimerPayload, rid)
				}
			}
		}
	}
}

func RunRoomCleanup(ctx context.Context, roomsService *rooms.Service, maxWait time.Duration) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = roomsService.CancelStaleRooms(ctx, maxWait)
		}
	}
}

// RunActiveMatchesReconcile removes orphaned IDs from matches:active (safety net for crashes).
func RunActiveMatchesReconcile(ctx context.Context, gamesService *games.Service) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = gamesService.ReconcileActiveMatches(ctx)
		}
	}
}

// RunTurnDeadlinesReconcile backfills turn_deadlines ZSET for active matches (handles pre-ZSET deploys).
func RunTurnDeadlinesReconcile(ctx context.Context, gamesService *games.Service) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = gamesService.ReconcileTurnDeadlines(ctx)
		}
	}
}
