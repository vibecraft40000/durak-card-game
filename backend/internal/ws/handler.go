package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
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

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type Handler struct {
	authService   *auth.Service
	rooms         *rooms.Service
	games         *games.Service
	wallet        *wallet.Service
	users         *users.Repository
	commissionBps int
	disableMoney  bool
	hub           *Hub
	bus           *Bus
	limiter       *ratelimit.Service
	upgrader      websocket.Upgrader
}

func NewHandler(authService *auth.Service, roomsService *rooms.Service, gamesService *games.Service, walletService *wallet.Service, usersRepo *users.Repository, commissionBps int, disableMoney bool, hub *Hub, bus *Bus, limiter *ratelimit.Service, allowedOrigin string) *Handler {
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
		authService:   authService,
		rooms:         roomsService,
		games:         gamesService,
		wallet:        walletService,
		users:         usersRepo,
		commissionBps: commissionBps,
		disableMoney:  disableMoney,
		hub:           hub,
		bus:           bus,
		limiter:       limiter,
		upgrader: websocket.Upgrader{
			CheckOrigin: checkOrigin,
		},
	}
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	userID, err := h.authService.ParseJWT(token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	roomID := r.URL.Query().Get("roomId")
	if roomID == "" {
		roomID = chi.URLParam(r, "id")
	}
	if roomID == "" {
		http.Error(w, "room id required", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	const maxMessageSize = 32 << 10
	conn.SetReadLimit(maxMessageSize)

	room, roomErr := h.rooms.Get(r.Context(), roomID)
	if roomErr != nil || !slices.Contains(room.Players, userID) {
		conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "forbidden_room"), time.Now().Add(5*time.Second))
		conn.Close()
		return
	}

	client := NewClient(userID, roomID, conn, 512)
	if !h.hub.Register(client) {
		conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "too_many_connections"), time.Now().Add(5*time.Second))
		conn.Close()
		return
	}
	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			h.hub.Unregister(client)
			client.Close()
			ctx := context.Background()
			room, roomErr := h.rooms.Get(ctx, client.RoomID)
			if roomErr == nil && room.Status == rooms.StatusInGame && room.MatchID != "" {
				_ = h.games.MarkDisconnected(ctx, room.MatchID, client.UserID)
				metrics.IncWSDisconnect("in_game_grace")
				h.broadcast(ctx, client.RoomID, ServerEvent{
					Type: "player_disconnected",
					Payload: map[string]any{
						"roomId":   client.RoomID,
						"playerId": client.UserID,
					},
				})
			} else {
				metrics.IncWSDisconnect("leave")
				if room, err := h.rooms.LeaveOnDisconnect(ctx, client.RoomID, client.UserID); err == nil {
					h.broadcast(ctx, client.RoomID, ServerEvent{Type: "room_update", Payload: room})
				}
			}
		})
	}
	defer cleanup()
	go client.writePump(5*time.Second, cleanup)

	h.broadcast(r.Context(), roomID, ServerEvent{
		Type: "room_update",
		Payload: func() any {
			room, err := h.rooms.Get(r.Context(), roomID)
			if err != nil {
				return map[string]string{"roomId": roomID, "connectedUserId": userID}
			}
			return room
		}(),
	})

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var event ClientEvent
		if err := json.Unmarshal(msg, &event); err != nil {
			h.sendRoomError(client, "invalid event payload")
			continue
		}
		h.handleClientEvent(r.Context(), client, event)
	}
}

func (h *Handler) handleClientEvent(ctx context.Context, client *Client, event ClientEvent) {
	switch event.Type {
	case "join_room", "reconnect":
		if !h.allowWSRateLimit(ctx, client, event.Type) {
			return
		}
		room, err := h.rooms.Get(ctx, client.RoomID)
		if err != nil {
			h.sendRoomError(client, err.Error())
			return
		}
		h.broadcast(ctx, client.RoomID, ServerEvent{
			Type:    "room_update",
			Payload: room,
		})
		if room.MatchID != "" {
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
				h.broadcast(ctx, client.RoomID, ServerEvent{
					Type:    "game_state",
					Payload: h.toGameStateDTO(ctx, client.RoomID, state),
				})
				h.handleBotTurns(ctx, client.RoomID, room, state)
			}
		}
	case "ready", "confirm_join":
		room, err := h.rooms.Ready(ctx, client.RoomID, client.UserID)
		if err != nil {
			h.sendRoomError(client, err.Error())
			return
		}
		h.broadcast(ctx, client.RoomID, ServerEvent{Type: "room_update", Payload: room})
		if room.MatchID != "" {
			state, err := h.games.GetState(ctx, room.MatchID)
			if err == nil {
				h.broadcast(ctx, client.RoomID, ServerEvent{Type: "game_state", Payload: h.toGameStateDTO(ctx, client.RoomID, state)})
				h.handleBotTurns(ctx, client.RoomID, room, state)
			}
		}
	case "start_game":
		room, err := h.rooms.StartGame(ctx, client.RoomID, client.UserID)
		if err != nil {
			h.sendRoomError(client, err.Error())
			return
		}
		h.broadcast(ctx, client.RoomID, ServerEvent{Type: "room_update", Payload: room})
		if room.MatchID != "" {
			state, err := h.games.GetState(ctx, room.MatchID)
			if err == nil {
				h.broadcast(ctx, client.RoomID, ServerEvent{Type: "game_state", Payload: h.toGameStateDTO(ctx, client.RoomID, state)})
				h.handleBotTurns(ctx, client.RoomID, room, state)
			}
		}
	case "make_move":
		allowed, rlErr := h.limiter.Allow(ctx, "make_move:"+client.UserID, 30, 10*time.Second)
		if rlErr != nil {
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
				metrics.IncVersionMismatch()
				// Handled separately: send game_state + version_mismatch, no error
				currentState, getErr := h.games.GetState(ctx, room.MatchID)
				if getErr == nil {
					_ = h.hub.Send(client, ServerEvent{Type: "game_state", Payload: h.toGameStateDTO(ctx, client.RoomID, currentState)})
					_ = h.hub.Send(client, ServerEvent{
						Type: "version_mismatch",
						Payload: map[string]any{
							"roomId":   client.RoomID,
							"action":   actionRaw,
							"cardId":   cardID,
							"actionId": actionID,
						},
					})
				}
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
		h.broadcast(ctx, client.RoomID, ServerEvent{
			Type: "move_applied",
			Payload: map[string]any{
				"roomId":   client.RoomID,
				"matchId":  room.MatchID,
				"eventId":  eventID,
				"playerId": client.UserID,
				"action":   action,
				"cardId":   cardID,
			},
		})
		h.broadcast(ctx, client.RoomID, ServerEvent{Type: "game_state", Payload: h.toGameStateDTO(ctx, client.RoomID, state)})
		h.broadcast(ctx, client.RoomID, ServerEvent{
			Type: "timer_update",
			Payload: map[string]any{
				"roomId":       client.RoomID,
				"turnPlayerId": state.TurnPlayerID,
				"turnEndsAt":   state.TurnEndsAt.UnixMilli(),
			},
		})
		if state.Status == engine.StatusFinished {
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
			payload := map[string]any{"roomId": client.RoomID, "winnerPlayerId": state.WinnerPlayer}
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
			return
		}
		h.handleBotTurns(ctx, client.RoomID, room, state)
	case "send_message":
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
	case "sync_request":
		if !h.allowWSRateLimit(ctx, client, "sync_request") {
			return
		}
		room, err := h.rooms.Get(ctx, client.RoomID)
		if err != nil || room.MatchID == "" {
			return
		}
		state, err := h.games.GetState(ctx, room.MatchID)
		if err == nil {
			_ = h.hub.Send(client, ServerEvent{Type: "game_state", Payload: h.toGameStateDTO(ctx, client.RoomID, state)})
		}
	default:
		h.sendRoomError(client, "unsupported event type")
	}
}

func normalizeAction(raw string) engine.Action {
	switch raw {
	case "attack_card":
		return engine.ActionAttack
	case "defend_card":
		return engine.ActionDefend
	case "take_cards":
		return engine.ActionTake
	case "pass_turn", "end_round":
		return engine.ActionPass
	default:
		return engine.Action(raw)
	}
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
		h.broadcast(ctx, roomID, ServerEvent{
			Type: "move_applied",
			Payload: map[string]any{
				"roomId":   roomID,
				"matchId":  room.MatchID,
				"eventId":  eventID,
				"playerId": current.TurnPlayerID,
				"action":   string(action),
				"cardId":   cardID,
			},
		})
		h.broadcast(ctx, roomID, ServerEvent{Type: "game_state", Payload: h.toGameStateDTO(ctx, roomID, nextState)})
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
			payload := map[string]any{"roomId": roomID, "winnerPlayerId": nextState.WinnerPlayer}
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
	h.broadcast(ctx, roomID, ServerEvent{
		Type: "move_applied",
		Payload: map[string]any{
			"roomId":   roomID,
			"matchId":  result.MatchID,
			"eventId":  eventID,
			"playerId": result.PlayerID,
			"action":   string(result.Action),
			"cardId":   result.CardID,
		},
	})
	h.broadcast(ctx, roomID, ServerEvent{Type: "game_state", Payload: h.toGameStateDTO(ctx, roomID, result.State)})
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
		payload := map[string]any{"roomId": roomID, "winnerPlayerId": result.State.WinnerPlayer}
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
		return
	}
	h.handleBotTurns(ctx, roomID, room, result.State)
}

func (h *Handler) broadcast(ctx context.Context, roomID string, event ServerEvent) {
	h.hub.Broadcast(roomID, event)
	if h.bus != nil {
		_ = h.bus.Publish(ctx, roomID, event)
	}
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
		errors.Is(err, engine.ErrCardDoesNotBeat), errors.Is(err, engine.ErrAttackCardDenied):
		return "INVALID_CARD"
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
	_ = h.hub.Send(client, ServerEvent{Type: "error", Payload: payload})
}

func (h *Handler) toGameStateDTO(ctx context.Context, roomID string, state engine.GameState) GameStateDTO {
	players := make([]PlayerDTO, 0, len(state.Hands))
	for _, playerID := range state.PlayerOrder {
		hand := state.Hands[playerID]
		displayName := "player-" + playerID[:4]
		photoURL := ""
		if rooms.IsBotPlayer(playerID) {
			displayName = "bot"
		} else if user, ok := h.users.GetByID(ctx, playerID); ok {
			if user.DisplayName != "" {
				displayName = user.DisplayName
			}
			photoURL = user.PhotoURL
		}
		players = append(players, PlayerDTO{
			ID:            playerID,
			Username:      displayName,
			DisplayName:   displayName,
			PhotoURL:      photoURL,
			HandCount:     len(hand),
			Hand:          hand,
			IsCurrentTurn: state.TurnPlayerID == playerID,
		})
	}
	var trumpCard *engine.Card
	if len(state.Deck) > 0 {
		c := state.Deck[len(state.Deck)-1]
		trumpCard = &c
	}
	phase := string(state.TurnState)
	if phase == "" {
		phase = "attack"
	}
	return GameStateDTO{
		RoomID:         roomID,
		MatchID:        state.MatchID,
		Version:        state.Version,
		Phase:          phase,
		Players:        players,
		TableCards:     state.TableCards,
		TrumpSuit:      string(state.Trump),
		TrumpCard:      trumpCard,
		TurnPlayerID:   state.TurnPlayerID,
		TurnEndsAt:     state.TurnEndsAt.UnixMilli(),
		Status:         string(state.Status),
		WinnerPlayerID: state.WinnerPlayer,
	}
}
