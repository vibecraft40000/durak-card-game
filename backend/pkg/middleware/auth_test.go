package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"durakonline/backend/internal/users"
)

type stubJWTParser struct {
	userID string
	err    error
}

func (s stubJWTParser) ParseJWT(token string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.userID, nil
}

type stubUserGetter struct {
	user users.User
	ok   bool
}

func (s stubUserGetter) GetByID(_ context.Context, userID string) (users.User, bool) {
	if !s.ok || s.user.ID != userID {
		return users.User{}, false
	}
	return s.user, true
}

func TestAuthJWTInvalidTokenUsesEnvelope(t *testing.T) {
	middleware := AuthJWT(stubJWTParser{err: errors.New("bad token")}, stubUserGetter{}, nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["code"] != "invalid_token" {
		t.Fatalf("expected code invalid_token, got %v", resp["code"])
	}
	if resp["message"] != "invalid token" {
		t.Fatalf("expected message invalid token, got %v", resp["message"])
	}
}
