package middleware

import (
	"context"
	"net/http"
	"strings"

	"durakonline/backend/internal/users"
	"durakonline/backend/pkg/httpapi"
	"durakonline/backend/pkg/logger"

	"go.uber.org/zap"
)

type contextKey string

const UserContextKey contextKey = "user"

type jwtParser interface {
	ParseJWT(token string) (string, error)
}

type userGetter interface {
	GetByID(ctx context.Context, userID string) (users.User, bool)
}

func AuthJWT(authService jwtParser, userRepo userGetter, log *zap.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLog := logger.WithRequest(log, r)
			authHeader := r.Header.Get("Authorization")
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" || token == authHeader {
				requestLog.Warn("auth middleware: missing bearer token")
				httpapi.WriteError(w, r, http.StatusUnauthorized, "missing_bearer_token", "missing bearer token", nil)
				return
			}

			userID, err := authService.ParseJWT(token)
			if err != nil {
				requestLog.Warn("auth middleware: invalid token", zap.Error(err))
				httpapi.WriteError(w, r, http.StatusUnauthorized, "invalid_token", "invalid token", nil)
				return
			}

			user, ok := userRepo.GetByID(r.Context(), userID)
			if !ok {
				requestLog.Warn("auth middleware: user not found", zap.String("user_id", userID))
				httpapi.WriteError(w, r, http.StatusUnauthorized, "user_not_found", "user not found", nil)
				return
			}
			if user.IsBanned {
				requestLog.Warn("auth middleware: user is banned", zap.String("user_id", userID))
				httpapi.WriteError(w, r, http.StatusForbidden, "user_banned", "user is banned", nil)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context) (users.User, bool) {
	user, ok := ctx.Value(UserContextKey).(users.User)
	return user, ok
}
