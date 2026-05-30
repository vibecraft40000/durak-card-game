package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"durakonline/backend/internal/auth"
	"durakonline/backend/internal/rooms"
	"durakonline/backend/pkg/metrics"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

const maxWSMessageSize = 32 << 10

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	roomID := requestedRoomID(r)
	if roomID == "" {
		http.Error(w, "room id required", http.StatusBadRequest)
		return
	}

	userID, err := h.resolveWSUserID(r.Context(), r, roomID)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgradeWSConnection(w, r)
	if err != nil {
		return
	}
	if !h.authorizeRoomConnection(r.Context(), conn, roomID, userID) {
		return
	}

	client, ok := h.registerWSClient(r, conn, roomID, userID)
	if !ok {
		return
	}
	cleanup := h.newConnectionCleanup(client)
	defer cleanup()

	go client.writePump(5*time.Second, cleanup)
	h.broadcastInitialRoomUpdate(r.Context(), roomID, userID)
	h.readClientEvents(r.Context(), conn, client)
}

func requestedRoomID(r *http.Request) string {
	if r == nil {
		return ""
	}
	roomID := r.URL.Query().Get("roomId")
	if roomID != "" {
		return roomID
	}
	return chi.URLParam(r, "id")
}

func (h *Handler) upgradeWSConnection(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(maxWSMessageSize)
	return conn, nil
}

func (h *Handler) authorizeRoomConnection(ctx context.Context, conn *websocket.Conn, roomID, userID string) bool {
	room, err := h.rooms.Get(ctx, roomID)
	if err != nil || !slices.Contains(room.Players, userID) {
		closeWSConnection(conn, websocket.ClosePolicyViolation, "forbidden_room")
		return false
	}
	return true
}

func (h *Handler) registerWSClient(r *http.Request, conn *websocket.Conn, roomID, userID string) (*Client, bool) {
	client := NewClient(userID, roomID, localeFromRequest(r), conn, 512)
	if !h.hub.Register(client) {
		closeWSConnection(conn, websocket.ClosePolicyViolation, "too_many_connections")
		return nil, false
	}
	return client, true
}

func (h *Handler) newConnectionCleanup(client *Client) func() {
	var cleanupOnce sync.Once
	return func() {
		cleanupOnce.Do(func() {
			h.hub.Unregister(client)
			client.Close()

			ctx := context.Background()
			room, err := h.rooms.Get(ctx, client.RoomID)
			if err == nil && room.Status == rooms.StatusInGame && room.MatchID != "" {
				_ = h.games.MarkDisconnected(ctx, room.MatchID, client.UserID)
				metrics.IncWSDisconnect("in_game_grace")
				h.broadcast(ctx, client.RoomID, ServerEvent{
					Type: "player_disconnected",
					Payload: map[string]any{
						"roomId":   client.RoomID,
						"playerId": client.UserID,
					},
				})
				return
			}

			if shouldPreserveRoomMembershipOnDisconnect(room.Status) {
				metrics.IncWSDisconnect("pregame_detach")
				return
			}

			metrics.IncWSDisconnect("leave")
			if room, err := h.rooms.LeaveOnDisconnect(ctx, client.RoomID, client.UserID); err == nil {
				h.broadcast(ctx, client.RoomID, ServerEvent{Type: "room_update", Payload: room})
			}
		})
	}
}

func shouldPreserveRoomMembershipOnDisconnect(status rooms.Status) bool {
	return status != rooms.StatusInGame
}

func (h *Handler) broadcastInitialRoomUpdate(ctx context.Context, roomID, userID string) {
	h.broadcast(ctx, roomID, ServerEvent{
		Type: "room_update",
		Payload: func() any {
			room, err := h.rooms.Get(ctx, roomID)
			if err != nil {
				return map[string]string{
					"roomId":          roomID,
					"connectedUserId": userID,
				}
			}
			return room
		}(),
	})
}

func (h *Handler) readClientEvents(ctx context.Context, conn *websocket.Conn, client *Client) {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		event, err := decodeClientEvent(msg)
		if err != nil {
			h.sendRoomError(client, "invalid event payload")
			continue
		}
		h.handleClientEvent(ctx, client, event)
	}
}

func decodeClientEvent(msg []byte) (ClientEvent, error) {
	var event ClientEvent
	if err := json.Unmarshal(msg, &event); err != nil {
		return ClientEvent{}, err
	}
	return event, nil
}

func closeWSConnection(conn *websocket.Conn, code int, text string) {
	if conn == nil {
		return
	}
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(5*time.Second))
	_ = conn.Close()
}

func (h *Handler) resolveWSUserID(ctx context.Context, r *http.Request, roomID string) (string, error) {
	ticket := strings.TrimSpace(r.URL.Query().Get("ticket"))
	if ticket != "" {
		return h.authService.ConsumeWSTicket(ctx, ticket, roomID)
	}

	// Legacy migration fallback: keep JWT-in-query temporarily until all clients and tooling use ws tickets.
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		return "", auth.ErrInvalidWSTicket
	}
	return h.authService.ParseJWT(token)
}
