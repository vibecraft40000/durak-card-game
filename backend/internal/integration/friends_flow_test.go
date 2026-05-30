package integration

import (
	"context"
	"errors"
	"testing"

	"durakonline/backend/internal/friends"
	"durakonline/backend/internal/users"
)

func TestFriendsFlowServiceAndRepository(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	usersRepo := users.NewRepository(pg)
	friendsRepo := friends.NewRepository(pg)
	friendsSvc := friends.NewService(friendsRepo, usersRepo)

	alice, err := usersRepo.GetOrCreateByTelegram(ctx, 880001, "alice_friend", "Alice", "Friend", "")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := usersRepo.GetOrCreateByTelegram(ctx, 880002, "bob_friend", "Bob", "Friend", "")
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	if err := friendsSvc.SendRequest(ctx, alice.ID, "@bob_friend"); err != nil {
		t.Fatalf("send request by username: %v", err)
	}
	if err := friendsSvc.SendRequest(ctx, alice.ID, "@bob_friend"); err != nil {
		t.Fatalf("duplicate send request must be idempotent: %v", err)
	}
	if err := friendsSvc.SendRequest(ctx, alice.ID, alice.ID); !errors.Is(err, friends.ErrSelfFriend) {
		t.Fatalf("expected self-add to be rejected, got %v", err)
	}
	if err := friendsSvc.SendRequest(ctx, alice.ID, "@missing_friend"); err == nil {
		t.Fatal("expected unknown user request to fail")
	}

	requests, err := friendsSvc.ListIncomingRequests(ctx, bob.ID)
	if err != nil {
		t.Fatalf("list incoming requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected exactly one incoming request, got %d", len(requests))
	}
	if requests[0].UserID != alice.ID || requests[0].FriendID != bob.ID {
		t.Fatalf("unexpected request payload: %+v", requests[0])
	}

	if err := friendsSvc.AcceptRequest(ctx, bob.ID, alice.ID); err != nil {
		t.Fatalf("accept request: %v", err)
	}
	if err := friendsSvc.AcceptRequest(ctx, bob.ID, alice.ID); err == nil {
		t.Fatal("expected duplicate accept to fail once request is consumed")
	}

	aliceFriends, err := friendsSvc.ListFriends(ctx, alice.ID)
	if err != nil {
		t.Fatalf("list alice friends: %v", err)
	}
	bobFriends, err := friendsSvc.ListFriends(ctx, bob.ID)
	if err != nil {
		t.Fatalf("list bob friends: %v", err)
	}
	if len(aliceFriends) != 2 || len(bobFriends) != 2 {
		t.Fatalf("expected symmetric friendship rows, got alice=%d bob=%d", len(aliceFriends), len(bobFriends))
	}

	if err := friendsSvc.RemoveFriend(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("remove friend: %v", err)
	}
	aliceFriends, err = friendsSvc.ListFriends(ctx, alice.ID)
	if err != nil {
		t.Fatalf("list alice friends after remove: %v", err)
	}
	bobFriends, err = friendsSvc.ListFriends(ctx, bob.ID)
	if err != nil {
		t.Fatalf("list bob friends after remove: %v", err)
	}
	if len(aliceFriends) != 0 || len(bobFriends) != 0 {
		t.Fatalf("expected no friends after remove, got alice=%d bob=%d", len(aliceFriends), len(bobFriends))
	}
}
