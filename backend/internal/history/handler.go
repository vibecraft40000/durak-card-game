package history

import (
	"encoding/json"
	"fmt"
	"net/http"

	"durakonline/backend/pkg/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	params := ParseListParams(r.URL.Query())
	resp, err := h.service.List(r.Context(), user.ID, params)
	if err != nil {
		http.Error(w, "failed to load history", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Calendar(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	monthStr := r.URL.Query().Get("month")
	if monthStr == "" {
		http.Error(w, "month required (YYYY-MM)", http.StatusBadRequest)
		return
	}
	var year, month int
	_, err := fmt.Sscanf(monthStr, "%d-%d", &year, &month)
	if err != nil || year < 2000 || year > 2100 || month < 1 || month > 12 {
		http.Error(w, "invalid month format", http.StatusBadRequest)
		return
	}
	days, err := h.service.Calendar(r.Context(), user.ID, year, month)
	if err != nil {
		http.Error(w, "failed to load calendar", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": days})
}
