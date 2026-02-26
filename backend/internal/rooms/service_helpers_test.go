package rooms

import "testing"

func TestBotHelpers(t *testing.T) {
	roomID := "room-123"
	botID := BotPlayerID(roomID)
	if botID != "bot:"+roomID {
		t.Fatalf("unexpected bot id: %s", botID)
	}
	if !IsBotPlayer(botID) {
		t.Fatalf("expected IsBotPlayer=true")
	}
	if IsBotPlayer("user-1") {
		t.Fatalf("expected IsBotPlayer=false for normal user")
	}
	if !ContainsBotPlayerIn([]string{"user-1", botID}) {
		t.Fatalf("expected ContainsBotPlayerIn=true")
	}
	if ContainsBotPlayerIn([]string{"user-1", "user-2"}) {
		t.Fatalf("expected ContainsBotPlayerIn=false")
	}
}
