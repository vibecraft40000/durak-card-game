package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"durakonline/backend/internal/auth"
	"durakonline/backend/internal/cryptopay"
	"durakonline/backend/internal/friends"
	"durakonline/backend/internal/games"
	"durakonline/backend/internal/history"
	"durakonline/backend/internal/payments"
	"durakonline/backend/internal/ratelimit"
	"durakonline/backend/internal/rooms"
	"durakonline/backend/internal/scheduler"
	"durakonline/backend/internal/transactions"
	"durakonline/backend/internal/users"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/internal/ws"
	"durakonline/backend/pkg/config"
	"durakonline/backend/pkg/logger"
	"durakonline/backend/pkg/metrics"
	customMiddleware "durakonline/backend/pkg/middleware"
	"durakonline/backend/pkg/storage"

	"github.com/go-chi/chi/v5"
	mw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	log, err := logger.New()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// Env sanity check
	log.Info("env check",
		zap.Bool("JWT_SECRET_loaded", len(os.Getenv("JWT_SECRET")) > 0),
		zap.Bool("TELEGRAM_BOT_TOKEN_loaded", len(os.Getenv("TELEGRAM_BOT_TOKEN")) > 0),
		zap.Bool("ALLOW_DEV_TELEGRAM_AUTH", os.Getenv("ALLOW_DEV_TELEGRAM_AUTH") == "true"),
	)
	if cfg.DisableMoney && cfg.Env == "production" {
		log.Fatal("DISABLE_MONEY=true in production is not allowed - abort")
	}
	if cfg.DisableMoney {
		log.Info("MODE: TEST (DISABLE_MONEY=true, HoldBet skipped)")
	}

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
	authService := auth.NewService(userRepo, redisClient, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, cfg.ReplayTTL, cfg.TelegramBotToken)
	authHandler := auth.NewHandler(cfg, authService)

	walletService := wallet.NewService(postgresPool, txRepo)
	gamesService := games.NewService(postgresPool, redisClient, 25*time.Second, cfg.MatchStateTTL)
	roomsRepo := rooms.NewRepository(redisClient)
	roomsService := rooms.NewService(roomsRepo, gamesService, walletService, cfg.CommissionBps, cfg.DisableMoney)
	roomsHandler := rooms.NewHandler(roomsService)
	limiter := ratelimit.NewService(redisClient)
	webappURL := cfg.AllowedOrigin
	if webappURL == "*" {
		webappURL = "https://durakonline.duckdns.org"
	}
	paymentsRepo := payments.NewRepository(postgresPool)
	cryptoPayHandler := cryptopay.NewHandler(cfg.CryptoPayAPIToken, cfg.CryptoPayTestnet, txRepo, redisClient, webappURL, log).
		WithPaymentsRepo(paymentsRepo)
	paymentsClient := payments.NewClient(cfg.WalletPayAPIKey)
	paymentsService := payments.NewService(postgresPool, paymentsRepo, paymentsClient, txRepo)
	paymentsHandler := payments.NewHandler(paymentsService, cfg.WalletPayAPIKey)

	historyRepo := history.NewRepository(postgresPool)
	historyService := history.NewService(historyRepo)
	historyHandler := history.NewHandler(historyService)

	friendsRepo := friends.NewRepository(postgresPool)
	friendsService := friends.NewService(friendsRepo, userRepo)
	friendsHandler := friends.NewHandler(friendsService, userRepo)

	hub := ws.NewHub()
	bus := ws.NewBus(redisClient, instanceID)
	wsHandler := ws.NewHandler(authService, roomsService, gamesService, walletService, userRepo, cfg.CommissionBps, cfg.DisableMoney, hub, bus, limiter)

	router := chi.NewRouter()
	router.Use(mw.RequestID)
	router.Use(mw.RealIP)
	router.Use(mw.Recoverer)
	router.Use(metrics.HTTPMiddleware)
	router.Use(jsonContentType)
	router.Use(cors(cfg.AllowedOrigin))

	router.Get("/health", healthHandler(postgresPool, redisClient, cfg))
	router.Get("/healthz", healthHandler(postgresPool, redisClient, cfg))
	router.Get("/api/config", func(w http.ResponseWriter, r *http.Request) {
		cryptoBotUsername := "CryptoBot"
		if cfg.CryptoPayTestnet || strings.HasPrefix(cfg.CryptoPayAPIToken, "test") {
			cryptoBotUsername = "CryptoTestnetBot"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cryptoBotUsername": cryptoBotUsername,
		})
	})
	router.Get("/live", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	router.Get("/ready", healthHandler(postgresPool, redisClient, cfg))
	router.Handle("/metrics", metrics.Handler())

	router.Post("/auth/telegram", rateLimit(limiter, "login", 10, time.Minute, func(r *http.Request) string {
		return r.RemoteAddr
	}, authHandler.TelegramAuth))
	router.Post("/auth/refresh", authHandler.Refresh)

	router.Group(func(protected chi.Router) {
		protected.Use(customMiddleware.AuthJWT(authService, userRepo))
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
					"currency":    user.Currency,
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
				Currency    string `json:"currency"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			updated, err := userRepo.UpdateSettings(r.Context(), user.ID, req.DisplayName, req.Currency)
			if err != nil {
				http.Error(w, "failed to update settings", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"settings": map[string]any{
					"displayName": updated.DisplayName,
					"currency":    updated.Currency,
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
		protected.Get("/api/rooms", roomsHandler.List)
		protected.Get("/api/rooms/{id}", roomsHandler.Get)
		protected.Post("/api/rooms", roomsHandler.Create)
		protected.Post("/api/rooms/{id}/join", rateLimit(limiter, "join_room", 20, time.Minute, func(r *http.Request) string {
			user, ok := customMiddleware.UserFromContext(r.Context())
			if !ok {
				return ""
			}
			return user.ID
		}, roomsHandler.Join))
		protected.Post("/api/rooms/{id}/ready", roomsHandler.Ready)
		protected.Post("/api/rooms/{id}/start", roomsHandler.Start)
		protected.Post("/api/rooms/{id}/leave", roomsHandler.Leave)
		protected.Post("/api/deposit/create", cryptoPayHandler.CreateDepositInvoice)
		protected.Post("/api/withdraw/create", cryptoPayHandler.CreateWithdraw(walletService))
		protected.Post("/api/payments/create", paymentsHandler.CreatePayment)
		protected.Get("/api/history", historyHandler.List)
		protected.Get("/api/history/calendar", historyHandler.Calendar)
		protected.Get("/api/friends", friendsHandler.List)
		protected.Get("/api/friends/requests", friendsHandler.Requests)
		protected.Post("/api/friends/request", friendsHandler.Request)
		protected.Post("/api/friends/accept", friendsHandler.Accept)
	})

	router.Post("/webhooks/cryptopay", cryptoPayHandler.Webhook)
	router.Post("/api/wallet/webhook", paymentsHandler.Webhook)
	router.Get("/ws", wsHandler.ServeWS)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go scheduler.RunMatchTimers(ctx, gamesService, roomsService, hub, wsHandler)
	go scheduler.RunDisconnectTimeouts(ctx, gamesService, roomsService, hub, walletService, cfg.CommissionBps, cfg.DisableMoney)
	go scheduler.RunRoomCleanup(ctx, roomsService, cfg.RoomWaitTimeout)
	go scheduler.RunActiveMatchesReconcile(ctx, gamesService)
	go scheduler.RunTurnDeadlinesReconcile(ctx, gamesService)
	go func() {
		_ = bus.Subscribe(ctx, func(roomID string, event ws.ServerEvent) {
			hub.Broadcast(roomID, event)
		})
	}()
	go func() {
		pprofServer := &http.Server{
			Addr:              ":" + cfg.PprofPort,
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("pprof server failed", zap.Error(err))
		}
	}()

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

func rateLimit(
	limiter *ratelimit.Service,
	scope string,
	limit int,
	window time.Duration,
	keyFn func(r *http.Request) string,
	next http.HandlerFunc,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := scope + ":" + keyFn(r)
		allowed, err := limiter.Allow(r.Context(), key, limit, window)
		if err != nil {
			http.Error(w, "rate limiter error", http.StatusServiceUnavailable)
			return
		}
		if !allowed {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
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

func cors(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowOrigin := origin
			if origin == "*" {
				reqOrigin := r.Header.Get("Origin")
				if reqOrigin != "" {
					allowOrigin = reqOrigin
				}
			}
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
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

func healthHandler(pg *pgxpool.Pool, redisClient *redis.Client, cfg config.Config) http.HandlerFunc {
	mode := "money"
	if cfg.DisableMoney {
		mode = "test"
	}
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
			"mode":     mode,
		})
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
