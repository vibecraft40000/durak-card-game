package scheduler

import (
	"context"
	"time"

	"durakonline/backend/internal/games"
	"durakonline/backend/internal/rooms"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/internal/ws"
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
				if !disableMoney && !rooms.ContainsBotPlayerIn(r.State.PlayerOrder) {
					_ = games.SettleIfFinished(ctx, walletService, r.State, room.Stake, commissionBps)
				}
				_, _ = roomsService.MarkRoomFinished(ctx, roomID)
				hub.Broadcast(roomID, ws.ServerEvent{
					Type: "match_finished",
					Payload: map[string]any{
						"roomId":         roomID,
						"winnerPlayerId": r.State.WinnerPlayer,
						"abandoned":      true,
					},
				})
			}
		}
	}
}

func RunMatchTimers(ctx context.Context, gamesService *games.Service, roomsService *rooms.Service, hub *ws.Hub) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = gamesService.HandleTimeouts(ctx)
			activeRooms, _ := roomsService.List(ctx)
			for _, room := range activeRooms {
				if room.MatchID == "" {
					continue
				}
				state, err := gamesService.GetState(ctx, room.MatchID)
				if err != nil {
					continue
				}
				hub.Broadcast(room.ID, ws.ServerEvent{
					Type: "timer_update",
					Payload: map[string]any{
						"roomId":       room.ID,
						"turnPlayerId": state.TurnPlayerID,
						"turnEndsAt":   state.TurnEndsAt.UnixMilli(),
					},
				})
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
