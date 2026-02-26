package friends

import (
	"encoding/json"
	"net/http"

	"durakonline/backend/internal/users"
	"durakonline/backend/pkg/middleware"
)

type Handler struct {
	service  *Service
	users    *users.Repository
	presence presenceProvider
}

type presenceProvider interface {
	IsUserOnline(userID string) bool
}

func NewHandler(service *Service, users *users.Repository, presence presenceProvider) *Handler {
	return &Handler{service: service, users: users, presence: presence}
}

type friendResp struct {
	ID        string      `json:"id"`
	UserID    string      `json:"userId"`
	FriendID  string      `json:"friendId"`
	Status    string      `json:"status"`
	IsOnline  bool        `json:"isOnline"`
	CreatedAt string      `json:"createdAt"`
	Friend    *users.User `json:"friend,omitempty"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	list, err := h.service.ListFriends(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to list friends", http.StatusInternalServerError)
		return
	}
	resp := make([]friendResp, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, f := range list {
		otherID := f.FriendID
		if otherID == user.ID {
			otherID = f.UserID
		}
		if _, ok := seen[otherID]; ok {
			continue
		}
		seen[otherID] = struct{}{}
		other, _ := h.users.GetByID(r.Context(), otherID)
		isOnline := false
		if h.presence != nil {
			isOnline = h.presence.IsUserOnline(otherID)
		}
		resp = append(resp, friendResp{
			ID:        f.ID,
			UserID:    f.UserID,
			FriendID:  f.FriendID,
			Status:    f.Status,
			IsOnline:  isOnline,
			CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Friend:    &other,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"friends": resp})
}

func (h *Handler) Request(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		FriendID string `json:"friendId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FriendID == "" {
		http.Error(w, "friendId required", http.StatusBadRequest)
		return
	}
	err := h.service.SendRequest(r.Context(), user.ID, req.FriendID)
	if err != nil {
		if err == ErrSelfFriend {
			http.Error(w, "cannot add yourself", http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Accept(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		RequestID string `json:"requestId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RequestID == "" {
		http.Error(w, "requestId required", http.StatusBadRequest)
		return
	}
	err := h.service.AcceptRequest(r.Context(), user.ID, req.RequestID)
	if err != nil {
		if err == ErrRequestNotFound {
			http.Error(w, "request not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to accept", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Requests(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	list, err := h.service.ListIncomingRequests(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to list requests", http.StatusInternalServerError)
		return
	}
	resp := make([]friendResp, 0, len(list))
	for _, f := range list {
		requester, _ := h.users.GetByID(r.Context(), f.UserID)
		isOnline := false
		if h.presence != nil {
			isOnline = h.presence.IsUserOnline(f.UserID)
		}
		resp = append(resp, friendResp{
			ID:        f.ID,
			UserID:    f.UserID,
			FriendID:  f.FriendID,
			Status:    f.Status,
			IsOnline:  isOnline,
			CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Friend:    &requester,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"requests": resp})
}

func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		FriendID string `json:"friendId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FriendID == "" {
		http.Error(w, "friendId required", http.StatusBadRequest)
		return
	}
	if err := h.service.RemoveFriend(r.Context(), user.ID, req.FriendID); err != nil {
		if err == ErrSelfFriend {
			http.Error(w, "cannot remove yourself", http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
