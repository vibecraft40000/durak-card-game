package rooms

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"durakonline/backend/internal/wallet"
	"durakonline/backend/pkg/httpapi"
	"durakonline/backend/pkg/logger"
	"durakonline/backend/pkg/middleware"

	"github.com/go-chi/chi/v5"
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

type createRoomRequest struct {
	Title       string  `json:"title"`
	Stake       float64 `json:"stake"`
	MaxPlayers  int     `json:"maxPlayers"`
	Deck        int     `json:"deck"`
	Mode        string  `json:"mode"`
	PlayWithBot bool    `json:"playWithBot"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	rooms, err := h.service.List(r.Context())
	if err != nil {
		requestLog.Error("rooms: list failed", zap.Error(err))
		httpapi.WriteError(w, r, http.StatusInternalServerError, "rooms_list_failed", "failed to list rooms", nil)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		requestLog.Warn("rooms: create unauthorized")
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	var req createRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		requestLog.Warn("rooms: create invalid request body", zap.String("user_id", user.ID), zap.Error(err))
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "invalid json body", map[string]any{
			"field": "body",
		})
		return
	}

	if req.MaxPlayers == 0 {
		req.MaxPlayers = 2
	}
	if req.MaxPlayers < 2 || req.MaxPlayers > 4 {
		req.MaxPlayers = 2
	}
	if req.Deck == 0 {
		req.Deck = 36
	}
	if req.Deck != 24 && req.Deck != 36 && req.Deck != 52 {
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
		requestLog.Error("rooms: create failed", zap.String("user_id", user.ID), zap.String("title", req.Title), zap.Error(err))
		statusCode, code, message := classifyRoomMutationError(err, "failed to create room")
		httpapi.WriteError(w, r, statusCode, code, message, nil)
		return
	}
	requestLog.Info("rooms: create succeeded", zap.String("user_id", user.ID), zap.String("room_id", room.ID), zap.String("mode", room.Mode), zap.Int("max_players", room.MaxPlayers))
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"room": room})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	roomID := chi.URLParam(r, "id")
	room, err := h.service.Get(r.Context(), roomID)
	if err != nil {
		statusCode, code, message := classifyRoomGetError(err)
		logRoomError(requestLog, "rooms: get failed", statusCode, roomID, "", err)
		httpapi.WriteError(w, r, statusCode, code, message, nil)
		return
	}
	requestLog.Info("rooms: get succeeded", zap.String("room_id", roomID), zap.String("status", string(room.Status)), zap.Int("players", len(room.Players)))
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"room": room})
}

func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		requestLog.Warn("rooms: join unauthorized")
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	roomID := chi.URLParam(r, "id")
	room, err := h.service.Join(r.Context(), roomID, user.ID)
	if err != nil {
		statusCode, code, message := classifyRoomMutationError(err, "failed to join room")
		logRoomError(requestLog, "rooms: join failed", statusCode, roomID, user.ID, err)
		httpapi.WriteError(w, r, statusCode, code, message, nil)
		return
	}
	requestLog.Info("rooms: join succeeded", zap.String("room_id", roomID), zap.String("user_id", user.ID), zap.Int("players", len(room.Players)))
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"room": room, "ok": true})
}

func (h *Handler) Leave(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		requestLog.Warn("rooms: leave unauthorized")
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	roomID := chi.URLParam(r, "id")
	room, err := h.service.LeaveOnDisconnect(r.Context(), roomID, user.ID)
	if err != nil {
		statusCode, code, message := classifyRoomMutationError(err, "failed to leave room")
		logRoomError(requestLog, "rooms: leave failed", statusCode, roomID, user.ID, err)
		httpapi.WriteError(w, r, statusCode, code, message, nil)
		return
	}
	requestLog.Info("rooms: leave succeeded", zap.String("room_id", roomID), zap.String("user_id", user.ID), zap.Int("players", len(room.Players)))
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"room": room, "ok": true})
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		requestLog.Warn("rooms: start unauthorized")
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	roomID := chi.URLParam(r, "id")
	room, err := h.service.StartGame(r.Context(), roomID, user.ID)
	if err != nil {
		statusCode, code, message := classifyRoomMutationError(err, "failed to start room")
		logRoomError(requestLog, "rooms: start failed", statusCode, roomID, user.ID, err)
		httpapi.WriteError(w, r, statusCode, code, message, nil)
		return
	}
	requestLog.Info("rooms: start succeeded", zap.String("room_id", roomID), zap.String("user_id", user.ID), zap.String("match_id", room.MatchID))
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"room": room, "ok": true})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		requestLog.Warn("rooms: ready unauthorized")
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	roomID := chi.URLParam(r, "id")
	room, err := h.service.Ready(r.Context(), roomID, user.ID)
	if err != nil {
		statusCode, code, message := classifyRoomMutationError(err, "failed to confirm room readiness")
		logRoomError(requestLog, "rooms: ready failed", statusCode, roomID, user.ID, err)
		httpapi.WriteError(w, r, statusCode, code, message, nil)
		return
	}
	requestLog.Info("rooms: ready succeeded", zap.String("room_id", roomID), zap.String("user_id", user.ID), zap.Int("ready_players", len(room.ReadyUsers)), zap.String("match_id", room.MatchID))
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"room": room, "ok": true})
}

func (h *Handler) ConfirmStake(w http.ResponseWriter, r *http.Request) {
	requestLog := logger.WithRequest(h.log, r)
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		requestLog.Warn("rooms: confirm stake unauthorized")
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	roomID := chi.URLParam(r, "id")
	room, err := h.service.ConfirmStake(r.Context(), roomID, user.ID)
	if err != nil {
		statusCode, code, message := classifyRoomMutationError(err, "failed to confirm stake")
		logRoomError(requestLog, "rooms: confirm stake failed", statusCode, roomID, user.ID, err)
		httpapi.WriteError(w, r, statusCode, code, message, nil)
		return
	}
	requestLog.Info("rooms: confirm stake succeeded", zap.String("room_id", roomID), zap.String("user_id", user.ID), zap.Int("confirmed_players", len(room.StakeConfirmedUsers)), zap.String("match_id", room.MatchID))
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"room": room, "ok": true})
}

func classifyRoomGetError(err error) (int, string, string) {
	switch {
	case errors.Is(err, ErrRoomNotFound):
		return http.StatusNotFound, "room_not_found", ErrRoomNotFound.Error()
	default:
		return http.StatusInternalServerError, "room_load_failed", "failed to load room"
	}
}

func classifyRoomMutationError(err error, fallbackMessage string) (int, string, string) {
	switch {
	case errors.Is(err, ErrRoomNotFound):
		return http.StatusBadRequest, "room_not_found", ErrRoomNotFound.Error()
	case errors.Is(err, ErrRoomFull):
		return http.StatusBadRequest, "room_full", ErrRoomFull.Error()
	case errors.Is(err, ErrNeedOpponent):
		return http.StatusBadRequest, "waiting_for_opponent", ErrNeedOpponent.Error()
	case errors.Is(err, ErrNotRoomCreator):
		return http.StatusBadRequest, "not_room_creator", ErrNotRoomCreator.Error()
	case errors.Is(err, ErrNotAllReady):
		return http.StatusBadRequest, "not_all_ready", ErrNotAllReady.Error()
	case errors.Is(err, ErrStartInProgress):
		return http.StatusBadRequest, "start_in_progress", ErrStartInProgress.Error()
	case errors.Is(err, ErrStakeConfirmRequired):
		return http.StatusBadRequest, "stake_confirmation_required", ErrStakeConfirmRequired.Error()
	case errors.Is(err, ErrStakeConfirmExpired):
		return http.StatusBadRequest, "stake_confirmation_expired", ErrStakeConfirmExpired.Error()
	case errors.Is(err, ErrNotRoomParticipant):
		return http.StatusBadRequest, "room_participant_required", ErrNotRoomParticipant.Error()
	case errors.Is(err, ErrInvalidStake):
		return http.StatusBadRequest, "invalid_stake", ErrInvalidStake.Error()
	case errors.Is(err, wallet.ErrInsufficientBalance):
		return http.StatusBadRequest, "insufficient_funds", wallet.ErrInsufficientBalance.Error()
	case errors.Is(err, errRoomLocked):
		return http.StatusBadRequest, "room_locked", errRoomLocked.Error()
	default:
		return http.StatusInternalServerError, "internal_error", fallbackMessage
	}
}

func logRoomError(log *zap.Logger, message string, statusCode int, roomID, userID string, err error) {
	fields := []zap.Field{
		zap.String("room_id", roomID),
		zap.Error(err),
		zap.Int("status_code", statusCode),
	}
	if userID != "" {
		fields = append(fields, zap.String("user_id", userID))
	}
	if statusCode >= http.StatusInternalServerError {
		log.Error(message, fields...)
		return
	}
	log.Warn(message, fields...)
}
