package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"durakonline/backend/pkg/config"
	"durakonline/backend/pkg/httpapi"
	"durakonline/backend/pkg/logger"

	"go.uber.org/zap"
)

type Handler struct {
	cfg     config.Config
	service *Service
	log     *zap.Logger
	nowFunc func() time.Time
}

func NewHandler(cfg config.Config, service *Service, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{cfg: cfg, service: service, log: log, nowFunc: time.Now}
}

type telegramAuthRequest struct {
	InitData string `json:"initData"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) TelegramAuth(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	var req telegramAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		requestLog.Warn("telegram auth: invalid request body", zap.Error(err))
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "invalid JSON body", map[string]any{
			"field": "body",
		})
		return
	}

	tgUser, hash, referralCode, err := ValidateInitData(req.InitData, h.cfg.TelegramBotToken, h.cfg.AllowDevTelegramAuth, h.cfg.InitDataMaxAge, h.nowFunc())
	if err != nil {
		statusCode, code, message := classifyTelegramAuthError(err)
		requestLog.Warn("telegram auth: init data rejected", zap.String("code", code), zap.Error(err))
		httpapi.WriteError(w, r, statusCode, code, message, nil)
		return
	}

	if !h.cfg.IsLocalDevelopment() && hash == "dev" {
		requestLog.Warn("telegram auth: dev auth is not allowed")
		httpapi.WriteError(w, r, http.StatusUnauthorized, "dev_auth_not_allowed", "dev auth is not allowed outside local development", nil)
		return
	}

	// Subscription gate: require channel membership when REQUIRED_CHANNEL_ID is set.
	if h.cfg.RequiredChannelID != "" && !(h.cfg.AllowDevTelegramAuth && hash == "dev") {
		if !CheckChannelMembership(r.Context(), h.cfg.TelegramBotToken, h.cfg.RequiredChannelID, tgUser.ID) {
			requestLog.Warn("telegram auth: subscription required", zap.Int64("telegram_id", tgUser.ID))
			httpapi.WriteError(w, r, http.StatusForbidden, "subscription_required",
				"subscription to the channel is required to access the app",
				map[string]any{"channelLink": h.cfg.SubscriptionChannelLink})
			return
		}
	}

	// Optional strict replay protection (single-use initData hashes).
	// Disabled by default because Telegram clients can reuse initData in one session.
	if h.cfg.StrictInitDataReplay {
		ok, markErr := h.service.MarkInitDataHashUsed(r.Context(), hash)
		if !(h.cfg.AllowDevTelegramAuth && hash == "dev") {
			if markErr != nil {
				requestLog.Error("telegram auth: replay storage failed", zap.Error(markErr))
				httpapi.WriteError(w, r, http.StatusInternalServerError, "replay_storage_error", "replay storage error", nil)
				return
			}
			if !ok {
				requestLog.Warn("telegram auth: replay attack detected")
				httpapi.WriteError(w, r, http.StatusUnauthorized, "replay_attack_detected", "replay attack detected", nil)
				return
			}
		}
	}

	user, accessToken, refreshToken, err := h.service.ExchangeTelegram(r.Context(), tgUser, referralCode)
	if err != nil {
		requestLog.Error("telegram auth: exchange failed", zap.Error(err))
		httpapi.WriteError(w, r, http.StatusInternalServerError, "auth_exchange_failed", "auth exchange failed", nil)
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"user":         user,
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		requestLog.Warn("auth refresh: invalid request body", zap.Error(err))
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "invalid JSON body", map[string]any{
			"field": "body",
		})
		return
	}
	accessToken, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) {
			requestLog.Warn("auth refresh: invalid refresh token")
			httpapi.WriteError(w, r, http.StatusUnauthorized, "invalid_refresh_token", "invalid refresh token", nil)
			return
		}
		requestLog.Error("auth refresh: refresh failed", zap.Error(err))
		httpapi.WriteError(w, r, http.StatusInternalServerError, "refresh_failed", "refresh failed", nil)
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"accessToken": accessToken})
}

func classifyTelegramAuthError(err error) (int, string, string) {
	switch {
	case errors.Is(err, ErrExpiredAuthDate):
		return http.StatusUnauthorized, "init_data_expired", err.Error()
	case errors.Is(err, ErrInvalidSignature), errors.Is(err, ErrMissingHash):
		return http.StatusUnauthorized, "invalid_init_data", err.Error()
	default:
		return http.StatusUnauthorized, "invalid_init_data", err.Error()
	}
}
