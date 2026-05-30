package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestWSTicketIssueAndConsumeSingleUse(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	service := NewService(nil, redisClient, "test-secret", 15*time.Minute, 24*time.Hour, time.Hour, "")
	ticket, err := service.IssueWSTicket(context.Background(), "user-1", "room-1")
	if err != nil {
		t.Fatalf("issue ws ticket: %v", err)
	}
	if ticket == "" {
		t.Fatal("expected non-empty ws ticket")
	}

	ttl := redisServer.TTL(wsTicketKey(ticket))
	if ttl <= 0 || ttl > wsTicketTTL {
		t.Fatalf("unexpected ws ticket ttl: %s", ttl)
	}

	userID, err := service.ConsumeWSTicket(context.Background(), ticket, "room-1")
	if err != nil {
		t.Fatalf("consume ws ticket: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("unexpected user id: got %s want user-1", userID)
	}

	_, err = service.ConsumeWSTicket(context.Background(), ticket, "room-1")
	if !errors.Is(err, ErrInvalidWSTicket) {
		t.Fatalf("expected ErrInvalidWSTicket on second consume, got %v", err)
	}
}

func TestWSTicketConsumeRejectsWrongRoom(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	service := NewService(nil, redisClient, "test-secret", 15*time.Minute, 24*time.Hour, time.Hour, "")
	ticket, err := service.IssueWSTicket(context.Background(), "user-1", "room-1")
	if err != nil {
		t.Fatalf("issue ws ticket: %v", err)
	}

	_, err = service.ConsumeWSTicket(context.Background(), ticket, "room-2")
	if !errors.Is(err, ErrInvalidWSTicket) {
		t.Fatalf("expected ErrInvalidWSTicket for wrong room, got %v", err)
	}

	_, err = service.ConsumeWSTicket(context.Background(), ticket, "room-1")
	if !errors.Is(err, ErrInvalidWSTicket) {
		t.Fatalf("expected single-use invalidation after wrong-room consume, got %v", err)
	}
}
