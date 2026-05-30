package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"durakonline/backend/pkg/config"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWT_RoundTrip(t *testing.T) {
	service := NewService(nil, nil, "test-secret", 15*time.Minute, 24*time.Hour, "")
	token, err := service.issueJWT("user-1", time.Minute)
	if err != nil {
		t.Fatalf("issue jwt failed: %v", err)
	}
	userID, err := service.ParseJWT(token)
	if err != nil {
		t.Fatalf("parse jwt failed: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("expected user id user-1, got %s", userID)
	}
}

func TestJWT_RejectsUnexpectedSigningMethod(t *testing.T) {
	service := NewService(nil, nil, "test-secret", 15*time.Minute, 24*time.Hour, "")
	claims := jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Minute).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign jwt failed: %v", err)
	}

	if _, err := service.ParseJWT(signed); err == nil {
		t.Fatal("expected ParseJWT to reject non-HS256 token")
	}
}

func TestTelegramAuthInvalidJSONUsesEnvelope(t *testing.T) {
	handler := NewHandler(config.Config{}, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/auth/telegram", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	handler.TelegramAuth(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["code"] != "invalid_json" {
		t.Fatalf("expected code invalid_json, got %v", resp["code"])
	}
	if resp["message"] != "invalid JSON body" {
		t.Fatalf("expected message invalid JSON body, got %v", resp["message"])
	}
}
