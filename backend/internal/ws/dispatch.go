package ws

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"durakonline/backend/internal/games"
	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/rooms"
	"durakonline/backend/pkg/metrics"
)

func (h *Handler) handleClientEvent(ctx context.Context, client *Client, event ClientEvent) {
	switch event.Type {
	case "join_room", "reconnect":
		h.handleJoinOrReconnect(ctx, client, event)
	case "ready", "confirm_join":
		h.handleReadyOrConfirmJoin(ctx, client)
	case "start_game":
		h.handleStartGame(ctx, client)
	case "confirm_stake":
		h.handleConfirmStake(ctx, client)
	case "make_move":
		h.handleMakeMove(ctx, client, event)
	case "send_message":
		h.handleSendMessage(ctx, client, event)
	case "sync_request":
		h.handleSyncRequest(ctx, client, event)
	default:
		h.sendRoomError(client, "unsupported event type")
	}
}

func (h *Handler) handleJoinOrReconnect(ctx context.Context, client *Client, event ClientEvent) {
	if !h.allowWSRateLimit(ctx, client, event.Type) {
		return
	}

	syncOptions := readSyncRequestOptions(event.Payload)
	room, err := h.rooms.Get(ctx, client.RoomID)
	if err != nil {
		h.sendRoomError(client, err.Error())
		return
	}

	h.broadcast(ctx, client.RoomID, ServerEvent{
		Type:    "room_update",
		Payload: room,
	})
	if room.MatchID == "" {
		return
	}

	if room.Status == rooms.StatusInGame {
		_ = h.games.ClearDisconnected(ctx, room.MatchID, client.UserID)
		metrics.IncGameReconnect()
		h.broadcast(ctx, client.RoomID, ServerEvent{
			Type: "player_reconnected",
			Payload: map[string]any{
				"roomId":   client.RoomID,
				"playerId": client.UserID,
			},
		})
	}

	state, err := h.games.GetState(ctx, room.MatchID)
	if err == nil {
		h.syncStateToClient(ctx, client, room, state, syncOptions)
		h.handleBotTurns(ctx, client.RoomID, room, state)
	}
}

func (h *Handler) handleReadyOrConfirmJoin(ctx context.Context, client *Client) {
	room, err := h.rooms.Ready(ctx, client.RoomID, client.UserID)
	if err != nil {
		h.sendRoomError(client, err.Error())
		return
	}

	h.broadcast(ctx, client.RoomID, ServerEvent{Type: "room_update", Payload: room})
	if room.MatchID == "" {
		return
	}

	state, err := h.games.GetState(ctx, room.MatchID)
	if err == nil {
		h.broadcastGameState(ctx, client.RoomID, state)
		h.handleBotTurns(ctx, client.RoomID, room, state)
	}
}

func (h *Handler) handleStartGame(ctx context.Context, client *Client) {
	room, err := h.rooms.StartGame(ctx, client.RoomID, client.UserID)
	if err != nil {
		h.sendRoomError(client, err.Error())
		return
	}

	h.broadcast(ctx, client.RoomID, ServerEvent{Type: "room_update", Payload: room})
	if room.MatchID == "" {
		return
	}

	state, err := h.games.GetState(ctx, room.MatchID)
	if err == nil {
		h.broadcastGameState(ctx, client.RoomID, state)
		h.handleBotTurns(ctx, client.RoomID, room, state)
	}
}

func (h *Handler) handleConfirmStake(ctx context.Context, client *Client) {
	room, err := h.rooms.ConfirmStake(ctx, client.RoomID, client.UserID)
	if err != nil {
		h.sendRoomError(client, err.Error())
		return
	}

	h.broadcast(ctx, client.RoomID, ServerEvent{Type: "room_update", Payload: room})
	if room.MatchID == "" {
		return
	}

	state, err := h.games.GetState(ctx, room.MatchID)
	if err == nil {
		h.broadcastGameState(ctx, client.RoomID, state)
		h.handleBotTurns(ctx, client.RoomID, room, state)
	}
}

func (h *Handler) handleMakeMove(ctx context.Context, client *Client, event ClientEvent) {
	allowed, err := h.limiter.Allow(ctx, "make_move:"+client.UserID, 30, 10*time.Second)
	if err != nil {
		h.sendRoomError(client, "rate limiter unavailable")
		return
	}
	if !allowed {
		h.sendRoomErrorWithCode(client, "rate limit exceeded", "RATE_LIMIT")
		return
	}

	moveStart := time.Now()
	defer metrics.ObserveMatchMoveDuration(moveStart)

	actionRaw, _ := event.Payload["action"].(string)
	action := normalizeAction(actionRaw)
	cardID, _ := event.Payload["cardId"].(string)

	var expectedVersion *int64
	if v, ok := event.Payload["expectedVersion"].(float64); ok {
		iv := int64(v)
		expectedVersion = &iv
	}
	actionID, _ := event.Payload["actionId"].(string)

	room, err := h.rooms.Get(ctx, client.RoomID)
	if err != nil || room.MatchID == "" {
		h.sendRoomError(client, "match is not active")
		return
	}

	state, applied, err := h.games.Apply(ctx, room.MatchID, client.UserID, action, cardID, expectedVersion, actionID)
	if err != nil {
		if errors.Is(err, games.ErrVersionMismatch) {
			h.sendVersionMismatch(ctx, client, room, actionRaw, cardID, actionID)
			return
		}
		code := errorCodeFromEngine(err)
		h.sendRoomErrorWithCode(client, err.Error(), code)
		return
	}
	if !applied {
		return
	}

	eventID := ""
	if state.Version > 0 {
		eventID = fmt.Sprintf("%s:v%d", room.MatchID, state.Version)
	}
	h.broadcastMoveApplied(ctx, client.RoomID, room.MatchID, state.Version, client.UserID, string(action), cardID, eventID)
	h.broadcastGameState(ctx, client.RoomID, state)
	h.broadcast(ctx, client.RoomID, ServerEvent{
		Type: "timer_update",
		Payload: map[string]any{
			"roomId":       client.RoomID,
			"turnPlayerId": state.TurnPlayerID,
			"turnEndsAt":   state.TurnEndsAt.UnixMilli(),
		},
	})

	if state.Status == engine.StatusFinished {
		h.handleFinishedMatch(ctx, client, room, state)
		return
	}
	h.handleBotTurns(ctx, client.RoomID, room, state)
}

func (h *Handler) sendVersionMismatch(ctx context.Context, client *Client, room rooms.Room, actionRaw, cardID, actionID string) {
	metrics.IncVersionMismatch()

	// Handled separately: send game_state + version_mismatch, no error
	currentState, err := h.games.GetState(ctx, room.MatchID)
	if err != nil {
		return
	}

	_ = h.send(client, ServerEvent{
		Type:    "game_state",
		Payload: h.toGameStateDTO(ctx, client.RoomID, currentState, client.UserID),
	})
	_ = h.send(client, ServerEvent{
		Type: "version_mismatch",
		Payload: map[string]any{
			"roomId":   client.RoomID,
			"action":   actionRaw,
			"cardId":   cardID,
			"actionId": actionID,
		},
	})
}

func (h *Handler) handleFinishedMatch(ctx context.Context, client *Client, room rooms.Room, state engine.GameState) {
	var payoutInfo *games.PayoutInfo
	if !h.disableMoney && !containsBotPlayer(state.PlayerOrder) {
		payoutInfo, _ = h.games.SettleMatchIfFinished(ctx, h.wallet, state, room.Stake, h.commissionBps)
	}
	if _, err := h.rooms.MarkRoomFinished(ctx, client.RoomID); err == nil {
		h.broadcast(ctx, client.RoomID, ServerEvent{Type: "room_update", Payload: func() any {
			r, _ := h.rooms.Get(ctx, client.RoomID)
			return r
		}()})
	}

	payload := map[string]any{
		"roomId":          client.RoomID,
		"winnerPlayerId":  state.WinnerPlayer,
		"winnerPlayerIds": state.WinnerPlayers,
		"isDraw":          state.IsDraw,
		"finishGroups":    state.FinishGroups,
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

	h.broadcast(ctx, client.RoomID, ServerEvent{Type: "match_finished", Payload: payload})
	h.clearReplayMoves(room.MatchID)
}

func (h *Handler) handleSendMessage(ctx context.Context, client *Client, event ClientEvent) {
	if !h.allowWSRateLimit(ctx, client, "send_message") {
		return
	}

	raw, _ := event.Payload["message"].(string)
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return
	}

	const maxChatLen = 512
	runes := []rune(msg)
	if len(runes) > maxChatLen {
		msg = string(runes[:maxChatLen])
	}

	h.broadcast(ctx, client.RoomID, ServerEvent{
		Type: "chat_message",
		Payload: map[string]string{
			"userId":  client.UserID,
			"message": msg,
		},
	})
}

func (h *Handler) handleSyncRequest(ctx context.Context, client *Client, event ClientEvent) {
	if !h.allowWSRateLimit(ctx, client, "sync_request") {
		return
	}

	room, err := h.rooms.Get(ctx, client.RoomID)
	if err != nil || room.MatchID == "" {
		return
	}

	state, err := h.games.GetState(ctx, room.MatchID)
	if err == nil {
		h.syncStateToClient(ctx, client, room, state, readSyncRequestOptions(event.Payload))
	}
}

func normalizeAction(raw string) engine.Action {
	switch raw {
	case "attack_card":
		return engine.ActionAttack
	case "defend_card":
		return engine.ActionDefend
	case "throw_card", "throw_in":
		return engine.ActionThrow
	case "translate":
		return engine.ActionTranslate
	case "shuler_play":
		return engine.ActionShulerPlay
	case "take_cards":
		return engine.ActionTake
	case "shuler_report":
		return engine.ActionShulerReport
	case "pass_turn", "end_round":
		return engine.ActionPass
	default:
		return engine.Action(raw)
	}
}

type syncRequestOptions struct {
	lastKnownVersion  *int64
	lastKnownMatchID  string
	supportsStateDiff bool
}

func readSyncRequestOptions(payload map[string]interface{}) syncRequestOptions {
	return syncRequestOptions{
		lastKnownVersion:  readLastKnownVersion(payload),
		lastKnownMatchID:  readPayloadString(payload, "lastKnownMatchId"),
		supportsStateDiff: readPayloadBool(payload, "supportsStateDiff"),
	}
}

func readPayloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}

	raw, ok := payload[key]
	if !ok || raw == nil {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func readPayloadBool(payload map[string]interface{}, key string) bool {
	if payload == nil {
		return false
	}

	raw, ok := payload[key]
	if !ok || raw == nil {
		return false
	}

	switch value := raw.(type) {
	case bool:
		return value
	case float64:
		return value != 0
	case int:
		return value != 0
	case int64:
		return value != 0
	case string:
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func readLastKnownVersion(payload map[string]interface{}) *int64 {
	if payload == nil {
		return nil
	}

	raw, ok := payload["lastKnownVersion"]
	if !ok || raw == nil {
		return nil
	}

	switch value := raw.(type) {
	case float64:
		v := int64(value)
		if v < 0 {
			v = 0
		}
		return &v
	case int64:
		v := value
		if v < 0 {
			v = 0
		}
		return &v
	case int:
		v := int64(value)
		if v < 0 {
			v = 0
		}
		return &v
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return nil
		}
		if parsed < 0 {
			parsed = 0
		}
		return &parsed
	default:
		return nil
	}
}
