package auth

import (
	"testing"
	"time"
)

func TestReplayKeyFormat(t *testing.T) {
	key := replayKey("abc")
	if key != "auth:replay:abc" {
		t.Fatalf("unexpected replay key: %s", key)
	}
}

func TestJWT_RoundTrip(t *testing.T) {
	service := NewService(nil, nil, "test-secret", 15*time.Minute, 24*time.Hour, time.Hour, "")
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
