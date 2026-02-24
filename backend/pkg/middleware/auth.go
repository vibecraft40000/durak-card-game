package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"durakonline/backend/internal/auth"
	"durakonline/backend/internal/users"
)

type contextKey string

const UserContextKey contextKey = "user"

func AuthJWT(authService *auth.Service, userRepo *users.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" || token == authHeader {
				log.Printf("[auth] 401 path=%s reason=missing_token", r.URL.Path)
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}

			userID, err := authService.ParseJWT(token)
			if err != nil {
				log.Printf("[auth] 401 path=%s reason=invalid_token err=%v", r.URL.Path, err)
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			user, ok := userRepo.GetByID(r.Context(), userID)
			if !ok {
				log.Printf("[auth] 401 path=%s reason=user_not_found user_id=%s", r.URL.Path, userID)
				http.Error(w, "user not found", http.StatusUnauthorized)
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
