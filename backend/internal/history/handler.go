package history

import (
	"fmt"
	"net/http"

	"durakonline/backend/pkg/httpapi"
	"durakonline/backend/pkg/logger"
	"durakonline/backend/pkg/middleware"

	"go.uber.org/zap"
)

type Handler struct {
	service *Service
	log     *zap.Logger
}

func NewHandler(service *Service, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{service: service, log: log}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		requestLog.Warn("history: list unauthorized")
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	params := ParseListParams(r.URL.Query())
	resp, err := h.service.List(r.Context(), user.ID, params)
	if err != nil {
		requestLog.Error("history: list failed", zap.String("user_id", user.ID), zap.Error(err))
		httpapi.WriteError(w, r, http.StatusInternalServerError, "history_list_failed", "failed to load history", nil)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Calendar(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		requestLog.Warn("history: calendar unauthorized")
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	monthStr := r.URL.Query().Get("month")
	if monthStr == "" {
		requestLog.Warn("history: calendar month missing", zap.String("user_id", user.ID))
		httpapi.WriteError(w, r, http.StatusBadRequest, "month_required", "month required (YYYY-MM)", map[string]any{
			"field": "month",
		})
		return
	}
	var year, month int
	_, err := fmt.Sscanf(monthStr, "%d-%d", &year, &month)
	if err != nil || year < 2000 || year > 2100 || month < 1 || month > 12 {
		requestLog.Warn("history: calendar month invalid", zap.String("user_id", user.ID), zap.String("month", monthStr), zap.Error(err))
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_month", "invalid month format", map[string]any{
			"field": "month",
			"value": monthStr,
		})
		return
	}
	days, err := h.service.Calendar(r.Context(), user.ID, year, month)
	if err != nil {
		requestLog.Error("history: calendar failed", zap.String("user_id", user.ID), zap.Int("year", year), zap.Int("month", month), zap.Error(err))
		httpapi.WriteError(w, r, http.StatusInternalServerError, "history_calendar_failed", "failed to load calendar", nil)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": days})
}
