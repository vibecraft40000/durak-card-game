package rooms

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"durakonline/backend/pkg/middleware"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type createRoomRequest struct {
	Title       string  `json:"title"`
	Stake       float64 `json:"stake"`
	MaxPlayers  int     `json:"maxPlayers"`
	Deck        int     `json:"deck"`
	Mode        string  `json:"mode"`
	PlayWithBot bool    `json:"playWithBot"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.service.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list rooms", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req createRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if req.MaxPlayers == 0 {
		req.MaxPlayers = 2
	}
	if req.MaxPlayers != 2 {
		req.MaxPlayers = 2
	}
	if req.Deck == 0 {
		req.Deck = 36
	}
	if req.Mode == "" {
		req.Mode = "Подкидной"
	}
	if req.PlayWithBot {
		req.MaxPlayers = 1
		req.Mode = "bot"
	}
	if req.Title == "" {
		req.Title = "Стол " + strconv.FormatFloat(req.Stake, 'f', 0, 64)
	}

	room, err := h.service.Create(r.Context(), req.Title, req.Stake, req.MaxPlayers, req.Deck, req.Mode, user.ID)
	if err != nil {
		log.Printf("createRoom failed user_id=%s title=%q err=%v", user.ID, req.Title, err)
		http.Error(w, "failed to create room", http.StatusInternalServerError)
		return
	}
	log.Printf("createRoom ok user_id=%s room_id=%s mode=%s max_players=%d", user.ID, room.ID, room.Mode, room.MaxPlayers)
	writeJSON(w, http.StatusCreated, map[string]any{"room": room})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")
	room, err := h.service.Get(r.Context(), roomID)
	if err != nil {
		log.Printf("getRoom failed room_id=%s err=%v", roomID, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	log.Printf("getRoom ok room_id=%s status=%s players=%d", roomID, room.Status, len(room.Players))
	writeJSON(w, http.StatusOK, map[string]any{"room": room})
}

func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	roomID := chi.URLParam(r, "id")
	room, err := h.service.Join(r.Context(), roomID, user.ID)
	if err != nil {
		log.Printf("joinRoom failed room_id=%s user_id=%s err=%v", roomID, user.ID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("joinRoom ok room_id=%s user_id=%s players=%d", roomID, user.ID, len(room.Players))
	writeJSON(w, http.StatusOK, map[string]any{"room": room, "ok": true})
}

func (h *Handler) Leave(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	roomID := chi.URLParam(r, "id")
	room, err := h.service.LeaveOnDisconnect(r.Context(), roomID, user.ID)
	if err != nil {
		log.Printf("leaveRoom failed room_id=%s user_id=%s err=%v", roomID, user.ID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("leaveRoom ok room_id=%s user_id=%s players=%d", roomID, user.ID, len(room.Players))
	writeJSON(w, http.StatusOK, map[string]any{"room": room, "ok": true})
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	roomID := chi.URLParam(r, "id")
	room, err := h.service.StartGame(r.Context(), roomID, user.ID)
	if err != nil {
		log.Printf("startGame failed room_id=%s user_id=%s err=%v", roomID, user.ID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("startGame ok room_id=%s user_id=%s match_id=%s", roomID, user.ID, room.MatchID)
	writeJSON(w, http.StatusOK, map[string]any{"room": room, "ok": true})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	roomID := chi.URLParam(r, "id")
	room, err := h.service.Ready(r.Context(), roomID, user.ID)
	if err != nil {
		log.Printf("confirmStart failed room_id=%s user_id=%s err=%v", roomID, user.ID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("confirmStart ok room_id=%s user_id=%s ready=%d match_id=%s", roomID, user.ID, len(room.ReadyUsers), room.MatchID)
	writeJSON(w, http.StatusOK, map[string]any{"room": room, "ok": true})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
