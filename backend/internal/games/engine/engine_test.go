package engine

import (
	"testing"
	"time"
)

func TestApplyAction_Attack_Deterministic(t *testing.T) {
	const seed int64 = 42
	state := NewGameStateDeterministic("m1", []string{"p1", "p2"}, 30*time.Second, seed)
	if state.TurnState != TurnAttack || state.TurnPlayerID != "p1" {
		t.Fatalf("expected attack phase, p1 turn; got %s, %s", state.TurnState, state.TurnPlayerID)
	}
	cardID := state.Hands["p1"][0].ID
	err := ApplyAction(&state, "p1", ActionAttack, cardID, 30*time.Second)
	if err != nil {
		t.Fatalf("ApplyAction attack: %v", err)
	}
	if len(state.TableCards) != 1 {
		t.Errorf("expected 1 card on table; got %d", len(state.TableCards))
	}
	if state.TurnState != TurnDefend || state.TurnPlayerID != "p2" {
		t.Errorf("expected defend phase, p2 turn; got %s, %s", state.TurnState, state.TurnPlayerID)
	}
	if state.Version != 2 {
		t.Errorf("expected version 2; got %d", state.Version)
	}
}

func TestApplyAction_Defend_Deterministic(t *testing.T) {
	const seed int64 = 123
	state := NewGameStateDeterministic("m1", []string{"p1", "p2"}, 30*time.Second, seed)
	attackCardID := state.Hands["p1"][0].ID
	if err := ApplyAction(&state, "p1", ActionAttack, attackCardID, 30*time.Second); err != nil {
		t.Fatalf("attack: %v", err)
	}
	hand := state.Hands["p2"]
	var defendCardID string
	for _, c := range hand {
		if beats(state.TableCards[0], c, state.Trump) {
			defendCardID = c.ID
			break
		}
	}
	if defendCardID == "" {
		t.Skip("no valid defense card in hand for this seed")
	}
	err := ApplyAction(&state, "p2", ActionDefend, defendCardID, 30*time.Second)
	if err != nil {
		t.Fatalf("ApplyAction defend: %v", err)
	}
	if len(state.TableCards) != 2 {
		t.Errorf("expected 2 cards on table; got %d", len(state.TableCards))
	}
	if state.TurnState != TurnAttack || state.TurnPlayerID != "p1" {
		t.Errorf("expected attack phase, p1 turn; got %s, %s", state.TurnState, state.TurnPlayerID)
	}
}

func TestApplyAction_InvalidTurn(t *testing.T) {
	state := NewGameStateDeterministic("m1", []string{"p1", "p2"}, 30*time.Second, 1)
	cardID := state.Hands["p2"][0].ID
	err := ApplyAction(&state, "p2", ActionAttack, cardID, 30*time.Second)
	if err != ErrInvalidTurn {
		t.Errorf("expected ErrInvalidTurn; got %v", err)
	}
}

func TestApplyAction_InvalidCardID(t *testing.T) {
	state := NewGameStateDeterministic("m1", []string{"p1", "p2"}, 30*time.Second, 1)
	err := ApplyAction(&state, "p1", ActionAttack, "nonexistent-id", 30*time.Second)
	if err != ErrAttackCardDenied && err != ErrCardMissing {
		t.Errorf("expected ErrAttackCardDenied or ErrCardMissing; got %v", err)
	}
}
