package games

import (
	"testing"

	"durakonline/backend/internal/games/engine"
)

func TestTimeoutActionForState_DefenderAfterThrowIn(t *testing.T) {
	svc := &Service{}
	state := engine.GameState{
		Status:       engine.StatusPlaying,
		TurnState:    engine.TurnDefend,
		TurnPlayerID: "p2",
		DefenderID:   "p2",
		TableCards: []engine.Card{
			{ID: "a1", Suit: engine.Clubs, Rank: "9"},
			{ID: "d1", Suit: engine.Hearts, Rank: "6"},
			{ID: "t3", Suit: engine.Spades, Rank: "9"},
		},
	}

	action, cardID := svc.timeoutActionForState(&state)
	if action != engine.ActionTake || cardID != "" {
		t.Fatalf("expected timeout take in defend phase after throw-in, got action=%s card=%q", action, cardID)
	}
}

func TestTimeoutActionForState_AttackerWindowAfterDefendedThrowIn(t *testing.T) {
	svc := &Service{}
	state := engine.GameState{
		Status:       engine.StatusPlaying,
		TurnState:    engine.TurnAttack,
		TurnPlayerID: "p1",
		AttackerID:   "p1",
		DefenderID:   "p2",
		TableCards: []engine.Card{
			{ID: "a1", Suit: engine.Clubs, Rank: "9"},
			{ID: "d1", Suit: engine.Hearts, Rank: "6"},
			{ID: "t3", Suit: engine.Spades, Rank: "9"},
			{ID: "d2", Suit: engine.Hearts, Rank: "7"},
		},
		Hands: map[string][]engine.Card{
			"p1": {{ID: "atk2", Suit: engine.Diamonds, Rank: "9"}},
		},
	}

	action, cardID := svc.timeoutActionForState(&state)
	if action != engine.ActionPass || cardID != "" {
		t.Fatalf("expected timeout pass in attacker window with defended pairs, got action=%s card=%q", action, cardID)
	}
}
