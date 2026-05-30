package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"durakonline/backend/internal/auth"
	"durakonline/backend/internal/friends"
	"durakonline/backend/internal/games"
	"durakonline/backend/internal/history"
	"durakonline/backend/internal/ratelimit"
	"durakonline/backend/internal/rooms"
	"durakonline/backend/internal/scheduler"
	"durakonline/backend/internal/transactions"
	"durakonline/backend/internal/users"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/internal/ws"
	"durakonline/backend/pkg/config"
	"durakonline/backend/pkg/httpapi"
	"durakonline/backend/pkg/logger"
	"durakonline/backend/pkg/metrics"
	customMiddleware "durakonline/backend/pkg/middleware"
	"durakonline/backend/pkg/storage"

	"github.com/go-chi/chi/v5"
	mw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	if err := loadDotEnvForLocalRuntime(); err != nil {
		panic(err)
	}
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "config validation failed:\n%v\n", err)
		os.Exit(1)
	}
	log, err := logger.New()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	log.Info("env check",
		zap.Bool("JWT_SECRET_loaded", len(os.Getenv("JWT_SECRET")) > 0),
		zap.Bool("TELEGRAM_BOT_TOKEN_loaded", len(os.Getenv("TELEGRAM_BOT_TOKEN")) > 0),
		zap.Bool("ALLOW_DEV_TELEGRAM_AUTH", os.Getenv("ALLOW_DEV_TELEGRAM_AUTH") == "true"),
	)

	postgresPool, err := storage.NewPostgresPool(context.Background(), cfg.PostgresURL)
	if err != nil {
		log.Fatal("postgres connect failed", zap.Error(err))
	}
	defer postgresPool.Close()

	redisClient, err := storage.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatal("redis connect failed", zap.Error(err))
	}
	defer redisClient.Close()

	if err := postgresPool.Ping(context.Background()); err != nil {
		log.Fatal("postgres ping failed", zap.Error(err))
	}
	if err := storage.PingRedis(context.Background(), redisClient); err != nil {
		log.Fatal("redis ping failed", zap.Error(err))
	}
	instanceID, _ := os.Hostname()
	if instanceID == "" {
		instanceID = fmt.Sprintf("api-%d", time.Now().UnixNano())
	}

	userRepo := users.NewRepository(postgresPool)
	txRepo := transactions.NewRepository(postgresPool)
	authService := auth.NewService(userRepo, redisClient, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, cfg.TelegramBotToken)
	authHandler := auth.NewHandler(cfg, authService, log)

	walletService := wallet.NewService(postgresPool, txRepo)
	gamesService := games.NewService(postgresPool, redisClient, 25*time.Second, cfg.MatchStateTTL)
	gamesService.SetDisconnectPolicy(games.NormalizeDisconnectPolicy(cfg.DisconnectPolicy))
	roomsRepo := rooms.NewRepository(redisClient)
	roomsService := rooms.NewService(roomsRepo, gamesService, walletService, cfg.CommissionBps, false)
	roomsHandler := rooms.NewHandler(roomsService, log)
	limiter := ratelimit.NewService(redisClient)

	historyRepo := history.NewRepository(postgresPool)
	historyService := history.NewService(historyRepo)
	historyHandler := history.NewHandler(historyService, log)

	hub := ws.NewHub()
	bus := ws.NewBus(redisClient, instanceID)

	friendsRepo := friends.NewRepository(postgresPool)
	friendsService := friends.NewService(friendsRepo, userRepo)
	friendsHandler := friends.NewHandler(friendsService, userRepo, hub)

	wsHandler := ws.NewHandler(authService, roomsService, gamesService, walletService, userRepo, cfg.CommissionBps, false, hub, bus, limiter, redisClient, cfg.AllowedOrigin, cfg.WSSyncDiffSkipFinalState)

	router := chi.NewRouter()
	router.Use(mw.RequestID)
	router.Use(mw.RealIP)
	router.Use(mw.Recoverer)
	router.Use(metrics.HTTPMiddleware)
	router.Use(jsonContentType)
	router.Use(cors(cfg.AllowedOrigin))

	router.Get("/health", healthHandler(postgresPool, redisClient))
	router.Get("/healthz", healthHandler(postgresPool, redisClient))
	router.Get("/api/config", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
		})
	})
	router.Get("/live", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	router.Get("/ready", healthHandler(postgresPool, redisClient))
	router.Handle("/metrics", metrics.Handler())

	if cfg.AdminSecret != "" {
		adminSecret := cfg.AdminSecret
		router.Get("/admin/stats", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Admin-Secret") != adminSecret {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			usersCount, err := userRepo.Count(r.Context())
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"users_count":    usersCount,
				"games_total":    0,
				"games_active":   0,
				"games_finished": 0,
			})
		})
		router.Get("/admin/users", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Admin-Secret") != adminSecret {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			if limit <= 0 {
				limit = 20
			}
			list, total, err := userRepo.ListPaginated(r.Context(), offset, limit)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"users": list, "total": total})
		})
		router.Post("/admin/users/{id}/ban", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Admin-Secret") != adminSecret {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			actor := adminActorFromRequest(r)
			if actor == "" {
				http.Error(w, "admin actor required", http.StatusForbidden)
				return
			}
			userID := chi.URLParam(r, "id")
			if userID == "" {
				http.Error(w, "user id required", http.StatusBadRequest)
				return
			}
			if _, ok := userRepo.GetByID(r.Context(), userID); !ok {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}
			if err := userRepo.SetBanned(r.Context(), userID, true); err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			_ = txRepo.AddAdminAudit(r.Context(), actor, "ban", userID, nil, "")
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user_id": userID, "is_banned": true})
		})
		router.Post("/admin/users/{id}/unban", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Admin-Secret") != adminSecret {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			actor := adminActorFromRequest(r)
			if actor == "" {
				http.Error(w, "admin actor required", http.StatusForbidden)
				return
			}
			userID := chi.URLParam(r, "id")
			if userID == "" {
				http.Error(w, "user id required", http.StatusBadRequest)
				return
			}
			if _, ok := userRepo.GetByID(r.Context(), userID); !ok {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}
			if err := userRepo.SetBanned(r.Context(), userID, false); err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			_ = txRepo.AddAdminAudit(r.Context(), actor, "unban", userID, nil, "")
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user_id": userID, "is_banned": false})
		})
		router.Get("/admin/logs", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Admin-Secret") != adminSecret {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			if limit <= 0 {
				limit = 100
			}
			logs, err := txRepo.ListOperationLogs(r.Context(), limit)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
		})
		router.Get("/admin/rooms/{id}/stake", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Admin-Secret") != adminSecret {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			roomID := chi.URLParam(r, "id")
			room, err := roomsService.Get(r.Context(), roomID)
			if err != nil {
				http.Error(w, "room not found", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"room_id":                room.ID,
				"status":                 room.Status,
				"players":                room.Players,
				"ready_users":            room.ReadyUsers,
				"stake_confirmed_users":  room.StakeConfirmedUsers,
				"stake_confirm_deadline": room.StakeConfirmDeadline,
				"match_id":               room.MatchID,
			})
		})
	}

	router.Post("/auth/telegram", rateLimit(log, limiter, "login", 10, time.Minute, func(r *http.Request) string {
		return requestIP(r)
	}, authHandler.TelegramAuth))
	router.Post("/auth/refresh", authHandler.Refresh)

	router.Group(func(protected chi.Router) {
		protected.Use(customMiddleware.AuthJWT(authService, userRepo, log))
		protected.Get("/api/profile", func(w http.ResponseWriter, r *http.Request) {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if user.PhotoURL == "" && user.TelegramID != 0 && cfg.TelegramBotToken != "" {
				if photoURL := auth.FetchUserPhotoURL(r.Context(), cfg.TelegramBotToken, user.TelegramID); photoURL != "" {
					if updated, err := userRepo.UpdatePhotoURL(r.Context(), user.ID, photoURL); err == nil {
						user = updated
					}
				}
			}
			balance, _ := txRepo.Balance(r.Context(), user.ID)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user":    user,
				"balance": balance,
			})
		})
		protected.Get("/api/user/settings", func(w http.ResponseWriter, r *http.Request) {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"settings": map[string]any{
					"displayName": user.DisplayName,
				},
				"user": user,
			})
		})
		protected.Patch("/api/user/settings", func(w http.ResponseWriter, r *http.Request) {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			var req struct {
				DisplayName string `json:"displayName"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			updated, err := userRepo.UpdateSettings(r.Context(), user.ID, req.DisplayName)
			if err != nil {
				http.Error(w, "failed to update settings", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"settings": map[string]any{
					"displayName": updated.DisplayName,
				},
				"user": updated,
			})
		})
		protected.Patch("/api/profile/language", func(w http.ResponseWriter, r *http.Request) {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			var req struct {
				Language string `json:"language"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			updated, err := userRepo.UpdateLanguage(r.Context(), user.ID, req.Language)
			if err != nil {
				http.Error(w, "failed to update language", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"user": updated})
		})
		protected.Patch("/api/profile/avatar", func(w http.ResponseWriter, r *http.Request) {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			var req struct {
				PhotoURL string `json:"photoUrl"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			updated, err := userRepo.UpdatePhotoURL(r.Context(), user.ID, req.PhotoURL)
			if err != nil {
				http.Error(w, "failed to update avatar", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"user": updated})
		})
		protected.Post("/api/ws-ticket", rateLimit(log, limiter, "ws_ticket", 30, time.Minute, func(r *http.Request) string {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				return ""
			}
			return user.ID
		}, func(w http.ResponseWriter, r *http.Request) {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			var req struct {
				RoomID string `json:"roomId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			req.RoomID = strings.TrimSpace(req.RoomID)
			if req.RoomID == "" {
				http.Error(w, "roomId is required", http.StatusBadRequest)
				return
			}

			ticket, err := authService.IssueWSTicket(r.Context(), user.ID, req.RoomID)
			if err != nil {
				http.Error(w, "failed to issue ws ticket", http.StatusInternalServerError)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"ticket":       ticket,
				"expiresInSec": int(authService.WSTicketTTL().Seconds()),
			})
		}))
		protected.Get("/api/rooms", roomsHandler.List)
		protected.Get("/api/rooms/{id}", roomsHandler.Get)
		protected.Post("/api/rooms", rateLimit(log, limiter, "create_room", 10, time.Minute, func(r *http.Request) string {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				return ""
			}
			return user.ID
		}, roomsHandler.Create))
		protected.Post("/api/rooms/{id}/join", rateLimit(log, limiter, "join_room", 20, time.Minute, func(r *http.Request) string {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				return ""
			}
			return user.ID
		}, roomsHandler.Join))
		protected.Post("/api/rooms/{id}/ready", roomsHandler.Ready)
		protected.Post("/api/rooms/{id}/stake/confirm", roomsHandler.ConfirmStake)
		protected.Post("/api/rooms/{id}/start", roomsHandler.Start)
		protected.Post("/api/rooms/{id}/leave", roomsHandler.Leave)
		protected.Get("/api/history", historyHandler.List)
		protected.Get("/api/history/calendar", historyHandler.Calendar)
		protected.Get("/api/referrals/summary", func(w http.ResponseWriter, r *http.Request) {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			stats, err := userRepo.GetReferralStats(r.Context(), user.ID, limit)
			if err != nil {
				http.Error(w, "failed to load referral stats", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"referralCode": user.ReferralCode,
				"stats":        stats,
			})
		})
		protected.Get("/api/friends", friendsHandler.List)
		protected.Get("/api/friends/requests", friendsHandler.Requests)
		protected.Post("/api/friends/request", friendsHandler.Request)
		protected.Post("/api/friends/accept", friendsHandler.Accept)
		protected.Post("/api/friends/remove", friendsHandler.Remove)
	})

	router.Get("/ws", wsHandler.ServeWS)

	server := &http.Server{
		Addr:           ":" + cfg.Port,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go scheduler.RunMatchTimers(ctx, gamesService, roomsService, hub, wsHandler)
	go scheduler.RunDisconnectTimeouts(ctx, gamesService, roomsService, hub, walletService, cfg.CommissionBps, false)
	go scheduler.RunRoomCleanup(ctx, roomsService, cfg.RoomWaitTimeout)
	go scheduler.RunStakeConfirmationTimeouts(ctx, roomsService, hub)
	go scheduler.RunActiveMatchesReconcile(ctx, gamesService)
	go scheduler.RunTurnDeadlinesReconcile(ctx, gamesService)
	go func() {
		_ = bus.Subscribe(ctx, func(roomID string, event ws.ServerEvent) {
			wsHandler.HandleBusEvent(ctx, roomID, event)
		})
	}()
	if cfg.Env != "production" {
		go func() {
			pprofServer := &http.Server{
				Addr:              "127.0.0.1:" + cfg.PprofPort,
				Handler:           http.DefaultServeMux,
				ReadHeaderTimeout: 5 * time.Second,
			}
			if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("pprof server failed", zap.Error(err))
			}
		}()
	}

	go func() {
		<-ctx.Done()
		wsHandler.Drain(5 * time.Second)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Info("API started", zap.String("port", cfg.Port))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("listen failed", zap.Error(err))
	}
}

func loadDotEnvForLocalRuntime() error {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	switch env {
	case "development", "dev", "local", "test":
		if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func rateLimit(
	log *zap.Logger,
	limiter *ratelimit.Service,
	scope string,
	limit int,
	window time.Duration,
	keyFn func(r *http.Request) string,
	next http.HandlerFunc,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestLog := logger.WithRequest(log, r)
		key := scope + ":" + keyFn(r)
		allowed, err := limiter.Allow(r.Context(), key, limit, window)
		if err != nil {
			requestLog.Error("rate limit: limiter unavailable", zap.String("scope", scope), zap.Error(err))
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "rate_limiter_error", "rate limiter error", nil)
			return
		}
		if !allowed {
			requestLog.Warn("rate limit: limit exceeded", zap.String("scope", scope))
			httpapi.WriteError(w, r, http.StatusTooManyRequests, "rate_limit_exceeded", "too many requests", nil)
			return
		}
		next(w, r)
	}
}

func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func cors(allowedOrigins string) func(http.Handler) http.Handler {
	origins := make(map[string]bool)
	for _, o := range strings.Split(allowedOrigins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins[o] = true
		}
	}
	allowAny := allowedOrigins == "*" || origins["*"]

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqOrigin := strings.TrimRight(r.Header.Get("Origin"), "/")
			var allowOrigin string
			if allowAny || (reqOrigin != "" && origins[reqOrigin]) {
				allowOrigin = reqOrigin
			} else if allowedOrigins != "" && !strings.Contains(allowedOrigins, ",") {
				allowOrigin = strings.TrimSpace(allowedOrigins)
			}
			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			}
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func healthHandler(pg *pgxpool.Pool, redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		pgStatus := "ok"
		redisStatus := "ok"
		status := "ok"

		if err := pg.Ping(ctx); err != nil {
			pgStatus = "degraded"
			status = "degraded"
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			redisStatus = "degraded"
			status = "degraded"
		}

		code := http.StatusOK
		if status != "ok" && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			code = http.StatusServiceUnavailable
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":   status,
			"postgres": pgStatus,
			"redis":    redisStatus,
		})
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	httpapi.WriteJSON(w, statusCode, payload)
}

func requestIP(r *http.Request) string {
	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return "unknown"
	}
	if ip, _, err := net.SplitHostPort(host); err == nil && ip != "" {
		return ip
	}
	return host
}

func adminActorFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	raw := strings.TrimSpace(r.Header.Get("X-Admin-Actor"))
	if raw == "" {
		return ""
	}
	actor := strings.Join(strings.Fields(raw), " ")
	if len(actor) > 128 {
		actor = actor[:128]
	}
	return actor
}
