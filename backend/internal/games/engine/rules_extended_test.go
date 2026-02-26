package engine

import (
	"fmt"
	"testing"
	"time"
)

func TestNewGameStateWithConfig_DeckSize(t *testing.T) {
	tests := []struct {
		deckSize int
		expected int
	}{
		{deckSize: 24, expected: 24},
		{deckSize: 36, expected: 36},
		{deckSize: 52, expected: 52},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("deck_%d", tc.deckSize), func(t *testing.T) {
			state := NewGameStateDeterministicWithConfig(
				"m1",
				[]string{"p1", "p2"},
				30*time.Second,
				GameConfig{DeckSize: tc.deckSize, Mode: "podkidnoy"},
				42,
			)

			totalCards := len(state.Deck)
			for _, playerID := range state.PlayerOrder {
				totalCards += len(state.Hands[playerID])
			}

			if totalCards != tc.expected {
				t.Fatalf("total cards mismatch: got %d want %d", totalCards, tc.expected)
			}
			if state.DeckType != tc.deckSize {
				t.Fatalf("deck type mismatch: got %d want %d", state.DeckType, tc.deckSize)
			}
		})
	}
}

func TestApplyAction_TranslateInPerevodnoy(t *testing.T) {
	state := NewGameStateDeterministicWithConfig(
		"m2",
		[]string{"p1", "p2", "p3"},
		30*time.Second,
		GameConfig{DeckSize: 36, Mode: "perevodnoy"},
		77,
	)

	state.AttackerID = "p1"
	state.DefenderID = "p2"
	state.TurnState = TurnDefend
	state.TurnPlayerID = "p2"
	state.TableCards = []Card{{ID: "attack", Suit: Hearts, Rank: "9"}}
	state.Hands["p2"] = []Card{{ID: "translate", Suit: Clubs, Rank: "9"}}

	err := ApplyAction(&state, "p2", ActionTranslate, "translate", 30*time.Second)
	if err != nil {
		t.Fatalf("translate failed: %v", err)
	}

	if state.DefenderID != "p3" {
		t.Fatalf("next defender mismatch: got %s want p3", state.DefenderID)
	}
	if state.TurnPlayerID != "p3" || state.TurnState != TurnDefend {
		t.Fatalf("turn mismatch after translate: state=%s player=%s", state.TurnState, state.TurnPlayerID)
	}
	if len(state.TableCards) != 2 {
		t.Fatalf("expected 2 table cards after translate, got %d", len(state.TableCards))
	}
}

func TestApplyAction_ShulerReportDisablesCheatDefense(t *testing.T) {
	baseState := func() GameState {
		return GameState{
			MatchID:        "m3",
			Status:         StatusPlaying,
			Version:        1,
			Trump:          Spades,
			AttackerID:     "p1",
			DefenderID:     "p2",
			TurnState:      TurnDefend,
			TurnPlayerID:   "p2",
			TableCards:     []Card{{ID: "attack", Suit: Hearts, Rank: "10"}},
			Hands: map[string][]Card{
				"p1": {{ID: "safe", Suit: Hearts, Rank: "J"}},
				"p2": {{ID: "bad", Suit: Clubs, Rank: "6"}},
			},
			PlayerOrder:    []string{"p1", "p2"},
			Metadata:       map[string]interface{}{"round_limit": 1},
			ShulerEnabled:  true,
			ShulerPlayerID: "p2",
		}
	}

	stateAllowed := baseState()
	if err := ApplyAction(&stateAllowed, "p2", ActionDefend, "bad", 30*time.Second); err != nil {
		t.Fatalf("cheater defend should pass before report, got: %v", err)
	}

	stateBlocked := baseState()
	if err := ApplyAction(&stateBlocked, "p1", ActionShulerReport, "", 30*time.Second); err != nil {
		t.Fatalf("shuler report failed: %v", err)
	}
	if !stateBlocked.ShulerDetected {
		t.Fatalf("expected shuler to be detected after report")
	}
	if err := ApplyAction(&stateBlocked, "p2", ActionDefend, "bad", 30*time.Second); err != ErrCardDoesNotBeat {
		t.Fatalf("expected ErrCardDoesNotBeat after report, got: %v", err)
	}
}

func TestMaybeFinish_DrawWhenAllPlayersEmpty(t *testing.T) {
	state := GameState{
		MatchID:     "m4",
		Status:      StatusPlaying,
		PlayerOrder: []string{"p1", "p2", "p3"},
		Hands: map[string][]Card{
			"p1": {},
			"p2": {},
			"p3": {},
		},
		Deck: []Card{},
	}

	maybeFinish(&state)

	if state.Status != StatusFinished {
		t.Fatalf("expected finished status, got %s", state.Status)
	}
	if !state.IsDraw {
		t.Fatalf("expected draw=true")
	}
	if len(state.WinnerPlayers) != 3 {
		t.Fatalf("expected 3 winners in draw, got %d", len(state.WinnerPlayers))
	}
}
