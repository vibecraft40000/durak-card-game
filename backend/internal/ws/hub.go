package ws

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"durakonline/backend/pkg/metrics"
	"github.com/gorilla/websocket"
)

type Client struct {
	UserID string
	RoomID string
	Conn   *websocket.Conn
	send   chan []byte
	closed atomic.Bool
}

func NewClient(userID, roomID string, conn *websocket.Conn, bufferSize int) *Client {
	return &Client{
		UserID: userID,
		RoomID: roomID,
		Conn:   conn,
		send:   make(chan []byte, bufferSize),
	}
}

func (c *Client) enqueue(raw []byte) bool {
	if c.closed.Load() {
		return false
	}
	select {
	case c.send <- raw:
		return true
	default:
		return false
	}
}

func (c *Client) writePump(writeDeadline time.Duration, onFailure func()) {
	for raw := range c.send {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(writeDeadline))
		if err := c.Conn.WriteMessage(websocket.TextMessage, raw); err != nil {
			if onFailure != nil {
				onFailure()
			}
			return
		}
	}
}

func (c *Client) Close() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}
	close(c.send)
	_ = c.Conn.Close()
}

type Hub struct {
	mu      sync.Mutex
	clients map[string]map[*Client]struct{}
	closed  atomic.Bool
}

func NewHub() *Hub {
	return &Hub{clients: make(map[string]map[*Client]struct{})}
}

func (h *Hub) Register(client *Client) {
	if h.closed.Load() {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client.RoomID]; !ok {
		h.clients[client.RoomID] = make(map[*Client]struct{})
	}
	h.clients[client.RoomID][client] = struct{}{}
	metrics.SetWSActiveConnections(h.countClientsLocked())
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if roomClients, ok := h.clients[client.RoomID]; ok {
		delete(roomClients, client)
		if len(roomClients) == 0 {
			delete(h.clients, client.RoomID)
		}
	}
	metrics.SetWSActiveConnections(h.countClientsLocked())
}

func (h *Hub) Drain(timeout time.Duration) {
	h.closed.Store(true)
	deadline := time.Now().Add(timeout)
	h.mu.Lock()
	clients := make([]*Client, 0, h.countClientsLocked())
	for _, roomClients := range h.clients {
		for client := range roomClients {
			clients = append(clients, client)
		}
	}
	h.mu.Unlock()

	for _, client := range clients {
		_ = client.Conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutdown"),
			deadline,
		)
		client.Close()
	}
}

func (h *Hub) Broadcast(roomID string, event ServerEvent) {
	h.mu.Lock()
	roomClients, ok := h.clients[roomID]
	if !ok {
		h.mu.Unlock()
		return
	}
	raw, _ := json.Marshal(event)
	slowClients := make([]*Client, 0)
	for client := range roomClients {
		if ok := client.enqueue(raw); !ok {
			slowClients = append(slowClients, client)
		}
	}
	h.mu.Unlock()

	for _, client := range slowClients {
		h.Unregister(client)
		client.Close()
	}
}

func (h *Hub) Send(client *Client, event ServerEvent) bool {
	raw, err := json.Marshal(event)
	if err != nil {
		return false
	}
	if ok := client.enqueue(raw); !ok {
		h.Unregister(client)
		client.Close()
		return false
	}
	return true
}

func (h *Hub) countClientsLocked() int {
	total := 0
	for _, roomClients := range h.clients {
		total += len(roomClients)
	}
	return total
}
