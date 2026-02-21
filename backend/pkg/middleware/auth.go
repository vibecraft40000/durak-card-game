package middleware

import (
	"context"
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
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}

			userID, err := authService.ParseJWT(token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			user, ok := userRepo.GetByID(r.Context(), userID)
			if !ok {
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
