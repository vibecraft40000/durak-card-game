package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"durakonline/backend/pkg/config"
)

type Handler struct {
	cfg     config.Config
	service *Service
	nowFunc func() time.Time
}

func NewHandler(cfg config.Config, service *Service) *Handler {
	return &Handler{cfg: cfg, service: service, nowFunc: time.Now}
}

type telegramAuthRequest struct {
	InitData string `json:"initData"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) TelegramAuth(w http.ResponseWriter, r *http.Request) {
	var req telegramAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	tgUser, hash, err := ValidateInitData(req.InitData, h.cfg.TelegramBotToken, h.cfg.AllowDevTelegramAuth, h.cfg.InitDataMaxAge, h.nowFunc())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	ok, markErr := h.service.MarkInitDataHashUsed(r.Context(), hash)
	if !(h.cfg.AllowDevTelegramAuth && hash == "dev") {
		if markErr != nil {
			http.Error(w, "replay storage error", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "replay attack detected", http.StatusUnauthorized)
			return
		}
	}

	user, accessToken, refreshToken, err := h.service.ExchangeTelegram(r.Context(), tgUser)
	if err != nil {
		http.Error(w, "auth exchange failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":         user,
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	accessToken, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) {
			http.Error(w, "invalid refresh token", http.StatusUnauthorized)
			return
		}
		http.Error(w, "refresh failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"accessToken": accessToken})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
