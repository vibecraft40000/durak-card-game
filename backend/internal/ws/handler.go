package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"sync"
	"time"

	"durakonline/backend/internal/auth"
	"durakonline/backend/internal/games"
	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/ratelimit"
	"durakonline/backend/internal/rooms"
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
	commissionBps int
	disableMoney  bool
	hub           *Hub
	bus           *Bus
	limiter       *ratelimit.Service
	upgrader      websocket.Upgrader
}

func NewHandler(authService *auth.Service, roomsService *rooms.Service, gamesService *games.Service, walletService *wallet.Service, commissionBps int, disableMoney bool, hub *Hub, bus *Bus, limiter *ratelimit.Service) *Handler {
	return &Handler{
		authService:   authService,
		rooms:         roomsService,
		games:         gamesService,
		wallet:        walletService,
		commissionBps: commissionBps,
		disableMoney:  disableMoney,
		hub:           hub,
		bus:           bus,
		limiter:       limiter,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
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

	client := NewClient(userID, roomID, conn, 512)
	h.hub.Register(client)
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
					Payload: toGameStateDTO(client.RoomID, state),
				})
				h.handleBotTurns(ctx, client.RoomID, room, state)
			}
		}
	case "ready", "confirm_join":
		log.Printf("confirmFlow received room_id=%s user_id=%s event=%s", client.RoomID, client.UserID, event.Type)
		room, err := h.rooms.Ready(ctx, client.RoomID, client.UserID)
		if err != nil {
			log.Printf("confirmFlow failed room_id=%s user_id=%s err=%v", client.RoomID, client.UserID, err)
			h.sendRoomError(client, err.Error())
			return
		}
		log.Printf("confirmFlow updated room_id=%s user_id=%s ready=%d players=%d match_id=%s", client.RoomID, client.UserID, len(room.ReadyUsers), len(room.Players), room.MatchID)
		h.broadcast(ctx, client.RoomID, ServerEvent{Type: "room_update", Payload: room})
		if room.MatchID != "" {
			state, err := h.games.GetState(ctx, room.MatchID)
			if err == nil {
				h.broadcast(ctx, client.RoomID, ServerEvent{Type: "game_state", Payload: toGameStateDTO(client.RoomID, state)})
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
			h.sendRoomError(client, "rate limit exceeded")
			return
		}
		moveStart := time.Now()
		defer metrics.ObserveMatchMoveDuration(moveStart)
		actionRaw, _ := event.Payload["action"].(string)
		action := normalizeAction(actionRaw)
		cardID, _ := event.Payload["cardId"].(string)
		room, err := h.rooms.Get(ctx, client.RoomID)
		if err != nil || room.MatchID == "" {
			h.sendRoomError(client, "match is not active")
			return
		}
		state, err := h.games.Apply(ctx, room.MatchID, client.UserID, action, cardID)
		if err != nil {
			h.sendRoomError(client, err.Error())
			return
		}
		h.broadcast(ctx, client.RoomID, ServerEvent{
			Type: "move_applied",
			Payload: map[string]any{
				"roomId":   client.RoomID,
				"matchId":  room.MatchID,
				"playerId": client.UserID,
				"action":   action,
				"cardId":   cardID,
			},
		})
		h.broadcast(ctx, client.RoomID, ServerEvent{Type: "game_state", Payload: toGameStateDTO(client.RoomID, state)})
		h.broadcast(ctx, client.RoomID, ServerEvent{
			Type: "timer_update",
			Payload: map[string]any{
				"roomId":       client.RoomID,
				"turnPlayerId": state.TurnPlayerID,
				"turnEndsAt":   state.TurnEndsAt.UnixMilli(),
			},
		})
		if state.Status == engine.StatusFinished {
			if !h.disableMoney && !containsBotPlayer(state.PlayerOrder) {
				_ = games.SettleIfFinished(ctx, h.wallet, state, room.Stake, h.commissionBps)
			}
			if _, err := h.rooms.MarkRoomFinished(ctx, client.RoomID); err == nil {
				h.broadcast(ctx, client.RoomID, ServerEvent{Type: "room_update", Payload: func() any {
					r, _ := h.rooms.Get(ctx, client.RoomID)
					return r
				}()})
			}
			h.broadcast(ctx, client.RoomID, ServerEvent{
				Type: "match_finished",
				Payload: map[string]any{
					"roomId":         client.RoomID,
					"winnerPlayerId": state.WinnerPlayer,
				},
			})
			return
		}
		h.handleBotTurns(ctx, client.RoomID, room, state)
	case "send_message":
		message, _ := event.Payload["message"].(string)
		h.broadcast(ctx, client.RoomID, ServerEvent{
			Type:    "chat_message",
			Payload: map[string]string{"userId": client.UserID, "message": message},
		})
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
		nextState, err := h.games.Apply(ctx, room.MatchID, current.TurnPlayerID, action, cardID)
		if err != nil {
			fallback := engine.ActionPass
			if current.TurnState == engine.TurnDefend {
				fallback = engine.ActionTake
			}
			nextState, err = h.games.Apply(ctx, room.MatchID, current.TurnPlayerID, fallback, "")
			if err != nil {
				return
			}
			action = fallback
			cardID = ""
		}
		h.broadcast(ctx, roomID, ServerEvent{
			Type: "move_applied",
			Payload: map[string]any{
				"roomId":   roomID,
				"matchId":  room.MatchID,
				"playerId": current.TurnPlayerID,
				"action":   string(action),
				"cardId":   cardID,
			},
		})
		h.broadcast(ctx, roomID, ServerEvent{Type: "game_state", Payload: toGameStateDTO(roomID, nextState)})
		h.broadcast(ctx, roomID, ServerEvent{
			Type: "timer_update",
			Payload: map[string]any{
				"roomId":       roomID,
				"turnPlayerId": nextState.TurnPlayerID,
				"turnEndsAt":   nextState.TurnEndsAt.UnixMilli(),
			},
		})
		if nextState.Status == engine.StatusFinished {
			if !h.disableMoney && !containsBotPlayer(nextState.PlayerOrder) {
				_ = games.SettleIfFinished(ctx, h.wallet, nextState, room.Stake, h.commissionBps)
			}
			_, _ = h.rooms.MarkRoomFinished(ctx, roomID)
			h.broadcast(ctx, roomID, ServerEvent{
				Type: "match_finished",
				Payload: map[string]any{
					"roomId":         roomID,
					"winnerPlayerId": nextState.WinnerPlayer,
				},
			})
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

func (h *Handler) broadcast(ctx context.Context, roomID string, event ServerEvent) {
	h.hub.Broadcast(roomID, event)
	if h.bus != nil {
		_ = h.bus.Publish(ctx, roomID, event)
	}
}

func (h *Handler) Drain(timeout time.Duration) {
	h.hub.Drain(timeout)
}

func (h *Handler) sendError(conn *websocket.Conn, message string) {
	raw, _ := json.Marshal(ServerEvent{
		Type:    "error",
		Payload: map[string]string{"message": message},
	})
	_ = conn.WriteMessage(websocket.TextMessage, raw)
}

func (h *Handler) sendRoomError(client *Client, message string) {
	_ = h.hub.Send(client, ServerEvent{
		Type:    "error",
		Payload: map[string]string{"message": message},
	})
}

func toGameStateDTO(roomID string, state engine.GameState) GameStateDTO {
	players := make([]PlayerDTO, 0, len(state.Hands))
	for _, playerID := range state.PlayerOrder {
		hand := state.Hands[playerID]
		username := "player-" + playerID[:4]
		if rooms.IsBotPlayer(playerID) {
			username = "bot"
		}
		players = append(players, PlayerDTO{
			ID:            playerID,
			Username:      username,
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
	return GameStateDTO{
		RoomID:         roomID,
		MatchID:        state.MatchID,
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
