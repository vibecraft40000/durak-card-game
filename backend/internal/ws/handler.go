package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"durakonline/backend/internal/auth"
	"durakonline/backend/internal/games"
	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/ratelimit"
	"durakonline/backend/internal/rooms"
	"durakonline/backend/internal/users"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/pkg/metrics"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	authService            *auth.Service
	rooms                  *rooms.Service
	games                  *games.Service
	wallet                 *wallet.Service
	users                  *users.Repository
	commissionBps          int
	disableMoney           bool
	syncDiffSkipFinalState bool
	hub                    *Hub
	bus                    *Bus
	limiter                *ratelimit.Service
	upgrader               websocket.Upgrader
	replayMu               sync.RWMutex
	replayMoves            map[string][]replayMoveEvent
	redis                  *redis.Client
}

type replayMoveEvent struct {
	Version int64
	Event   ServerEvent
}

type replayRedisEntry struct {
	Version int64       `json:"version"`
	Event   ServerEvent `json:"event"`
}

const (
	replayBufferPerMatch = 64
	replayRedisTTL       = 6 * time.Hour
	replaySyncTailMax    = 8
)

func NewHandler(authService *auth.Service, roomsService *rooms.Service, gamesService *games.Service, walletService *wallet.Service, usersRepo *users.Repository, commissionBps int, disableMoney bool, hub *Hub, bus *Bus, limiter *ratelimit.Service, redisClient *redis.Client, allowedOrigin string, syncDiffSkipFinalState bool) *Handler {
	allowed := make(map[string]bool)
	for _, o := range strings.Split(allowedOrigin, ",") {
		o = strings.TrimSpace(strings.TrimRight(o, "/"))
		if o != "" {
			allowed[o] = true
		}
	}
	checkOrigin := func(r *http.Request) bool {
		if allowedOrigin == "" || allowedOrigin == "*" || allowed["*"] {
			return true
		}
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		return origin != "" && allowed[origin]
	}

	return &Handler{
		authService:            authService,
		rooms:                  roomsService,
		games:                  gamesService,
		wallet:                 walletService,
		users:                  usersRepo,
		commissionBps:          commissionBps,
		disableMoney:           disableMoney,
		syncDiffSkipFinalState: syncDiffSkipFinalState,
		hub:                    hub,
		bus:                    bus,
		limiter:                limiter,
		upgrader: websocket.Upgrader{
			CheckOrigin: checkOrigin,
		},
		replayMoves: make(map[string][]replayMoveEvent),
		redis:       redisClient,
	}
}

func (h *Handler) syncStateToClient(ctx context.Context, client *Client, room rooms.Room, state engine.GameState, options syncRequestOptions) {
	fromVersion := int64(0)
	if options.lastKnownVersion != nil && *options.lastKnownVersion > 0 {
		fromVersion = *options.lastKnownVersion
	}

	mode := "noop"
	replayCount := 0
	replayFromVersion := int64(0)
	var replayEvents []ServerEvent
	if state.Version > fromVersion {
		mode = "snapshot"
		replayFrom := fromVersion
		// Version 1 is match bootstrap and has no move_applied event.
		if replayFrom < 1 {
			replayFrom = 1
		}
		if replayFrom < state.Version {
			fullReplay, ok := h.replayForVersionRange(room.MatchID, replayFrom, state.Version)
			if ok && len(fullReplay) > 0 {
				mode = "replay"
				replayEvents = fullReplay
				replayCount = len(fullReplay)
				replayFromVersion = replayFrom + 1
			} else {
				tailReplay, tailFromVersion, tailOK := h.replayTailForVersionRange(room.MatchID, replayFrom, state.Version, replaySyncTailMax)
				if tailOK && len(tailReplay) > 0 {
					replayEvents = tailReplay
					replayCount = len(tailReplay)
					replayFromVersion = tailFromVersion
				}
			}
		}
	}

	syncPayload := map[string]any{
		"roomId":      room.ID,
		"matchId":     room.MatchID,
		"fromVersion": fromVersion,
		"toVersion":   state.Version,
		"mode":        mode,
		"replayCount": replayCount,
	}
	if replayFromVersion > 0 {
		syncPayload["replayFromVersion"] = replayFromVersion
	}

	_ = h.send(client, ServerEvent{
		Type:    "state_sync",
		Payload: syncPayload,
	})
	metrics.ObserveWSSyncPayloadBytes("state_sync", payloadSizeBytes(syncPayload))
	metrics.IncWSStateSync(mode)
	metrics.AddWSReplayMovesSent(replayCount)
	for _, replayEvent := range replayEvents {
		_ = h.send(client, replayEvent)
		metrics.ObserveWSSyncPayloadBytes("move_applied", payloadSizeBytes(replayEvent.Payload))
	}
	sentStateDiff := false
	if mode == "snapshot" && fromVersion > 0 {
		stateDiffPayload := h.toStateDiffPayload(ctx, room.ID, state, client.UserID, fromVersion)
		_ = h.send(client, ServerEvent{
			Type:    "state_diff",
			Payload: stateDiffPayload,
		})
		metrics.IncWSStateDiff()
		metrics.ObserveWSSyncPayloadBytes("state_diff", payloadSizeBytes(stateDiffPayload))
		sentStateDiff = true
	}
	if h.shouldSkipFinalGameState(state, options, sentStateDiff) {
		metrics.IncWSSyncFinalStateSkipped()
		return
	}

	gameStatePayload := h.toGameStateDTO(ctx, room.ID, state, client.UserID)
	_ = h.send(client, ServerEvent{
		Type:    "game_state",
		Payload: gameStatePayload,
	})
	metrics.ObserveWSSyncPayloadBytes("game_state", payloadSizeBytes(gameStatePayload))
}

func (h *Handler) shouldSkipFinalGameState(state engine.GameState, options syncRequestOptions, sentStateDiff bool) bool {
	if !h.syncDiffSkipFinalState || !sentStateDiff || !options.supportsStateDiff {
		return false
	}
	if options.lastKnownMatchID == "" {
		return false
	}
	return options.lastKnownMatchID == state.MatchID
}

func payloadSizeBytes(payload any) int {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0
	}
	return len(raw)
}

func (h *Handler) replayForVersionRange(matchID string, fromVersion, toVersion int64) ([]ServerEvent, bool) {
	if matchID == "" || toVersion <= fromVersion {
		return nil, true
	}
	if replay, ok := h.replayForVersionRangeLocal(matchID, fromVersion, toVersion); ok {
		return replay, true
	}
	return h.replayForVersionRangeRedis(matchID, fromVersion, toVersion)
}

func (h *Handler) replayTailForVersionRange(matchID string, fromVersion, toVersion int64, maxTail int) ([]ServerEvent, int64, bool) {
	if matchID == "" || toVersion <= fromVersion || maxTail <= 0 {
		return nil, 0, false
	}
	if replay, replayFromVersion, ok := h.replayTailForVersionRangeLocal(matchID, fromVersion, toVersion, maxTail); ok {
		return replay, replayFromVersion, true
	}
	return h.replayTailForVersionRangeRedis(matchID, fromVersion, toVersion, maxTail)
}

func (h *Handler) replayForVersionRangeLocal(matchID string, fromVersion, toVersion int64) ([]ServerEvent, bool) {
	h.replayMu.RLock()
	buffer := slices.Clone(h.replayMoves[matchID])
	h.replayMu.RUnlock()
	if len(buffer) == 0 {
		return nil, false
	}
	return extractReplayFromBuffer(buffer, fromVersion, toVersion)
}

func (h *Handler) replayTailForVersionRangeLocal(matchID string, fromVersion, toVersion int64, maxTail int) ([]ServerEvent, int64, bool) {
	h.replayMu.RLock()
	buffer := slices.Clone(h.replayMoves[matchID])
	h.replayMu.RUnlock()
	if len(buffer) == 0 {
		return nil, 0, false
	}
	return extractReplayTailFromBuffer(buffer, fromVersion, toVersion, maxTail)
}

func (h *Handler) replayForVersionRangeRedis(matchID string, fromVersion, toVersion int64) ([]ServerEvent, bool) {
	if h.redis == nil {
		return nil, false
	}
	rawEntries, err := h.redis.LRange(context.Background(), replayRedisKey(matchID), 0, -1).Result()
	if err != nil || len(rawEntries) == 0 {
		return nil, false
	}
	buffer := make([]replayMoveEvent, 0, len(rawEntries))
	for _, raw := range rawEntries {
		var entry replayRedisEntry
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			continue
		}
		if entry.Version <= 1 || entry.Event.Type != "move_applied" {
			continue
		}
		buffer = append(buffer, replayMoveEvent{
			Version: entry.Version,
			Event:   entry.Event,
		})
	}
	if len(buffer) == 0 {
		return nil, false
	}
	replay, ok := extractReplayFromBuffer(buffer, fromVersion, toVersion)
	if !ok {
		return nil, false
	}
	h.replayMu.Lock()
	h.replayMoves[matchID] = append([]replayMoveEvent(nil), buffer...)
	h.replayMu.Unlock()
	return replay, true
}

func (h *Handler) replayTailForVersionRangeRedis(matchID string, fromVersion, toVersion int64, maxTail int) ([]ServerEvent, int64, bool) {
	if h.redis == nil {
		return nil, 0, false
	}
	rawEntries, err := h.redis.LRange(context.Background(), replayRedisKey(matchID), 0, -1).Result()
	if err != nil || len(rawEntries) == 0 {
		return nil, 0, false
	}
	buffer := make([]replayMoveEvent, 0, len(rawEntries))
	for _, raw := range rawEntries {
		var entry replayRedisEntry
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			continue
		}
		if entry.Version <= 1 || entry.Event.Type != "move_applied" {
			continue
		}
		buffer = append(buffer, replayMoveEvent{
			Version: entry.Version,
			Event:   entry.Event,
		})
	}
	if len(buffer) == 0 {
		return nil, 0, false
	}
	replay, replayFromVersion, ok := extractReplayTailFromBuffer(buffer, fromVersion, toVersion, maxTail)
	if !ok {
		return nil, 0, false
	}
	h.replayMu.Lock()
	h.replayMoves[matchID] = append([]replayMoveEvent(nil), buffer...)
	h.replayMu.Unlock()
	return replay, replayFromVersion, true
}

func extractReplayFromBuffer(buffer []replayMoveEvent, fromVersion, toVersion int64) ([]ServerEvent, bool) {
	expected := fromVersion + 1
	out := make([]ServerEvent, 0, toVersion-fromVersion)
	for _, item := range buffer {
		if item.Version < expected {
			continue
		}
		if item.Version > toVersion {
			break
		}
		if item.Version != expected {
			return nil, false
		}
		out = append(out, item.Event)
		expected++
	}
	if expected-1 != toVersion {
		return nil, false
	}
	return out, true
}

func extractReplayTailFromBuffer(buffer []replayMoveEvent, fromVersion, toVersion int64, maxTail int) ([]ServerEvent, int64, bool) {
	if len(buffer) == 0 || toVersion <= fromVersion || maxTail <= 0 {
		return nil, 0, false
	}

	expected := toVersion
	reversed := make([]ServerEvent, 0, maxTail)
	replayFromVersion := int64(0)

	for i := len(buffer) - 1; i >= 0 && len(reversed) < maxTail; i-- {
		item := buffer[i]
		if item.Version > expected {
			continue
		}
		if item.Version <= fromVersion {
			break
		}
		if item.Version != expected {
			break
		}
		reversed = append(reversed, item.Event)
		replayFromVersion = item.Version
		expected--
	}
	if len(reversed) == 0 {
		return nil, 0, false
	}

	out := make([]ServerEvent, len(reversed))
	for i := 0; i < len(reversed); i++ {
		out[i] = reversed[len(reversed)-1-i]
	}
	return out, replayFromVersion, true
}

func (h *Handler) recordReplayMove(matchID string, version int64, event ServerEvent, persistToRedis bool) {
	if matchID == "" || version <= 1 || event.Type != "move_applied" {
		return
	}
	h.recordReplayMoveLocal(matchID, version, event)
	if persistToRedis {
		h.persistReplayMoveRedis(matchID, version, event)
	}
}

func (h *Handler) recordReplayMoveLocal(matchID string, version int64, event ServerEvent) {
	h.replayMu.Lock()
	defer h.replayMu.Unlock()

	buffer := append(h.replayMoves[matchID], replayMoveEvent{
		Version: version,
		Event:   event,
	})
	if len(buffer) > replayBufferPerMatch {
		buffer = buffer[len(buffer)-replayBufferPerMatch:]
	}
	h.replayMoves[matchID] = buffer
}

func (h *Handler) persistReplayMoveRedis(matchID string, version int64, event ServerEvent) {
	if h.redis == nil {
		return
	}
	entry := replayRedisEntry{
		Version: version,
		Event:   event,
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	key := replayRedisKey(matchID)
	pipe := h.redis.TxPipeline()
	pipe.RPush(context.Background(), key, raw)
	pipe.LTrim(context.Background(), key, -replayBufferPerMatch, -1)
	pipe.Expire(context.Background(), key, replayRedisTTL)
	_, _ = pipe.Exec(context.Background())
}

func (h *Handler) clearReplayMoves(matchID string) {
	if matchID == "" {
		return
	}
	h.replayMu.Lock()
	delete(h.replayMoves, matchID)
	h.replayMu.Unlock()
	if h.redis != nil {
		_ = h.redis.Del(context.Background(), replayRedisKey(matchID)).Err()
	}
}

func (h *Handler) recordReplayFromEvent(event ServerEvent) {
	matchID, version, ok := extractReplayMeta(event)
	if !ok {
		return
	}
	h.recordReplayMove(matchID, version, event, false)
}

func extractReplayMeta(event ServerEvent) (string, int64, bool) {
	if event.Type != "move_applied" {
		return "", 0, false
	}
	raw, err := json.Marshal(event.Payload)
	if err != nil {
		return "", 0, false
	}
	var payload struct {
		MatchID string `json:"matchId"`
		EventID string `json:"eventId"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", 0, false
	}
	version := parseVersionFromEventID(payload.EventID)
	if payload.MatchID == "" || version <= 1 {
		return "", 0, false
	}
	return payload.MatchID, version, true
}

func parseVersionFromEventID(eventID string) int64 {
	if eventID == "" {
		return 0
	}
	index := strings.LastIndex(eventID, ":v")
	if index < 0 || index+2 >= len(eventID) {
		return 0
	}
	version, err := strconv.ParseInt(eventID[index+2:], 10, 64)
	if err != nil || version < 0 {
		return 0
	}
	return version
}

func replayRedisKey(matchID string) string {
	return "ws:replay:" + matchID
}

func (h *Handler) broadcastMoveApplied(ctx context.Context, roomID, matchID string, version int64, playerID, action, cardID, eventID string) {
	event := ServerEvent{
		Type: "move_applied",
		Payload: map[string]any{
			"roomId":   roomID,
			"matchId":  matchID,
			"eventId":  eventID,
			"playerId": playerID,
			"action":   action,
			"cardId":   cardID,
		},
	}
	h.recordReplayMove(matchID, version, event, true)
	h.broadcast(ctx, roomID, event)
}

func (h *Handler) handleBotTurns(ctx context.Context, roomID string, room rooms.Room, state engine.GameState) {
	const maxBotChainMoves = 4
	current := state
	for i := 0; i < maxBotChainMoves; i++ {
		if current.Status != engine.StatusPlaying || !rooms.IsBotPlayer(current.TurnPlayerID) {
			return
		}
		action, cardID := chooseBotMove(current)
		nextState, _, err := h.games.Apply(ctx, room.MatchID, current.TurnPlayerID, action, cardID, nil, "")
		if err != nil {
			fallback := engine.ActionPass
			if current.TurnState == engine.TurnDefend {
				fallback = engine.ActionTake
			}
			nextState, _, err = h.games.Apply(ctx, room.MatchID, current.TurnPlayerID, fallback, "", nil, "")
			if err != nil {
				return
			}
			action = fallback
			cardID = ""
		}
		eventID := ""
		if nextState.Version > 0 {
			eventID = fmt.Sprintf("%s:v%d", room.MatchID, nextState.Version)
		}
		h.broadcastMoveApplied(ctx, roomID, room.MatchID, nextState.Version, current.TurnPlayerID, string(action), cardID, eventID)
		h.broadcastGameState(ctx, roomID, nextState)
		h.broadcast(ctx, roomID, ServerEvent{
			Type: "timer_update",
			Payload: map[string]any{
				"roomId":       roomID,
				"turnPlayerId": nextState.TurnPlayerID,
				"turnEndsAt":   nextState.TurnEndsAt.UnixMilli(),
			},
		})
		if nextState.Status == engine.StatusFinished {
			var payoutInfo *games.PayoutInfo
			if !h.disableMoney && !containsBotPlayer(nextState.PlayerOrder) {
				payoutInfo, _ = h.games.SettleMatchIfFinished(ctx, h.wallet, nextState, room.Stake, h.commissionBps)
			}
			_, _ = h.rooms.MarkRoomFinished(ctx, roomID)
			payload := map[string]any{
				"roomId":          roomID,
				"winnerPlayerId":  nextState.WinnerPlayer,
				"winnerPlayerIds": nextState.WinnerPlayers,
				"isDraw":          nextState.IsDraw,
				"finishGroups":    nextState.FinishGroups,
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
			h.broadcast(ctx, roomID, ServerEvent{Type: "match_finished", Payload: payload})
			h.clearReplayMoves(room.MatchID)
			return
		}
		current = nextState
	}
}

func chooseBotMove(state engine.GameState) (engine.Action, string) {
	hand := state.Hands[state.TurnPlayerID]
	if len(hand) == 0 {
		if state.TurnState == engine.TurnDefend {
			return engine.ActionTake, ""
		}
		return engine.ActionPass, ""
	}
	switch state.TurnState {
	case engine.TurnDefend:
		return engine.ActionDefend, hand[0].ID
	default:
		return engine.ActionAttack, hand[0].ID
	}
}

func containsBotPlayer(playerIDs []string) bool {
	return slices.ContainsFunc(playerIDs, rooms.IsBotPlayer)
}

// BroadcastTimeoutApplied broadcasts move_applied, game_state, timer_update for a timeout-applied move.
// If match finished, also broadcasts room_update and match_finished (with settlement).
func (h *Handler) BroadcastTimeoutApplied(ctx context.Context, roomID string, result *games.TimeoutResult, room rooms.Room) {
	eventID := ""
	if result.State.Version > 0 {
		eventID = fmt.Sprintf("%s:v%d", result.MatchID, result.State.Version)
	}
	h.broadcastMoveApplied(ctx, roomID, result.MatchID, result.State.Version, result.PlayerID, string(result.Action), result.CardID, eventID)
	h.broadcastGameState(ctx, roomID, result.State)
	h.broadcast(ctx, roomID, ServerEvent{
		Type: "timer_update",
		Payload: map[string]any{
			"roomId":       roomID,
			"turnPlayerId": result.State.TurnPlayerID,
			"turnEndsAt":   result.State.TurnEndsAt.UnixMilli(),
		},
	})
	if result.State.Status == engine.StatusFinished {
		var payoutInfo *games.PayoutInfo
		if !h.disableMoney && !containsBotPlayer(result.State.PlayerOrder) {
			payoutInfo, _ = h.games.SettleMatchIfFinished(ctx, h.wallet, result.State, room.Stake, h.commissionBps)
		}
		if _, err := h.rooms.MarkRoomFinished(ctx, roomID); err == nil {
			h.broadcast(ctx, roomID, ServerEvent{Type: "room_update", Payload: func() any {
				r, _ := h.rooms.Get(ctx, roomID)
				return r
			}()})
		}
		payload := map[string]any{
			"roomId":          roomID,
			"winnerPlayerId":  result.State.WinnerPlayer,
			"winnerPlayerIds": result.State.WinnerPlayers,
			"isDraw":          result.State.IsDraw,
			"finishGroups":    result.State.FinishGroups,
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
		h.broadcast(ctx, roomID, ServerEvent{Type: "match_finished", Payload: payload})
		h.clearReplayMoves(result.MatchID)
		return
	}
	h.handleBotTurns(ctx, roomID, room, result.State)
}

func (h *Handler) broadcastGameStateLocal(ctx context.Context, roomID string, state engine.GameState) {
	clients := h.hub.SnapshotRoomClients(roomID)
	for _, client := range clients {
		_ = h.send(client, ServerEvent{
			Type:    "game_state",
			Payload: h.toGameStateDTO(ctx, roomID, state, client.UserID),
		})
	}
}

func (h *Handler) broadcastGameState(ctx context.Context, roomID string, state engine.GameState) {
	h.broadcastGameStateLocal(ctx, roomID, state)
	if h.bus != nil {
		_ = h.bus.Publish(ctx, roomID, ServerEvent{Type: "game_state_internal", Payload: state})
	}
}

func (h *Handler) HandleBusEvent(ctx context.Context, roomID string, event ServerEvent) {
	if event.Type == "game_state_internal" {
		raw, err := json.Marshal(event.Payload)
		if err != nil {
			return
		}
		var state engine.GameState
		if err := json.Unmarshal(raw, &state); err != nil {
			return
		}
		h.broadcastGameStateLocal(ctx, roomID, state)
		return
	}
	if event.Type == "move_applied" {
		h.recordReplayFromEvent(event)
	}
	h.hub.Broadcast(roomID, withServerEventMeta(event, ""))
}

func (h *Handler) broadcast(ctx context.Context, roomID string, event ServerEvent) {
	event = withServerEventMeta(event, "")
	h.hub.Broadcast(roomID, event)
	if h.bus != nil {
		_ = h.bus.Publish(ctx, roomID, event)
	}
}

func (h *Handler) send(client *Client, event ServerEvent) bool {
	return h.hub.Send(client, withServerEventMeta(event, client.Locale))
}

func (h *Handler) Drain(timeout time.Duration) {
	h.hub.Drain(timeout)
}

const (
	wsRateLimitChatPer10s  = 15
	wsRateLimitSyncPer10s  = 5
	wsRateLimitJoinPer10s  = 3
	wsRateLimitOtherPer10s = 20
)

func wsLimitForEvent(eventType string) int {
	switch eventType {
	case "send_message":
		return wsRateLimitChatPer10s
	case "sync_request":
		return wsRateLimitSyncPer10s
	case "join_room", "reconnect":
		return wsRateLimitJoinPer10s
	default:
		return wsRateLimitOtherPer10s
	}
}

func (h *Handler) allowWSRateLimit(ctx context.Context, client *Client, eventType string) bool {
	key := fmt.Sprintf("ws:%s:%s", client.UserID, eventType)
	limit := wsLimitForEvent(eventType)
	allowed, err := h.limiter.Allow(ctx, key, limit, 10*time.Second)
	if err != nil {
		h.sendRoomError(client, "rate limiter unavailable")
		return false
	}
	if !allowed {
		h.sendRoomErrorWithCode(client, "rate limit exceeded", "RATE_LIMIT")
		return false
	}
	return true
}

func (h *Handler) sendError(conn *websocket.Conn, message string) {
	raw, _ := json.Marshal(ServerEvent{
		Type:    "error",
		Payload: map[string]string{"message": message},
	})
	_ = conn.WriteMessage(websocket.TextMessage, raw)
}

func errorCodeFromEngine(err error) string {
	switch {
	case errors.Is(err, engine.ErrInvalidTurn):
		return "INVALID_TURN"
	case errors.Is(err, engine.ErrCardMissing), errors.Is(err, engine.ErrInvalidMove),
		errors.Is(err, engine.ErrCardDoesNotBeat), errors.Is(err, engine.ErrAttackCardDenied),
		errors.Is(err, engine.ErrTranslateDenied), errors.Is(err, engine.ErrThrowDenied):
		return "INVALID_CARD"
	case errors.Is(err, engine.ErrCannotReportShuler), errors.Is(err, engine.ErrShulerPlayDenied):
		return "INVALID_ACTION"
	default:
		return ""
	}
}

func (h *Handler) sendRoomError(client *Client, message string) {
	h.sendRoomErrorWithCode(client, message, "")
}

func (h *Handler) sendRoomErrorWithCode(client *Client, message string, errorCode string) {
	payload := map[string]any{"message": message}
	if errorCode != "" {
		payload["errorCode"] = errorCode
	}
	_ = h.send(client, ServerEvent{Type: "error", Payload: payload})
}
