package engine

import (
	"slices"
	"testing"
	"time"
)

func TestViewAffordances_AttackerWindow(t *testing.T) {
	state := GameState{
		MatchID:      "m-aff-attack",
		Status:       StatusPlaying,
		Version:      3,
		Trump:        Spades,
		AttackerID:   "p1",
		DefenderID:   "p2",
		TurnState:    TurnAttack,
		TurnPlayerID: "p1",
		TableCards: []Card{
			{ID: "a1", Suit: Hearts, Rank: "9"},
			{ID: "d1", Suit: Hearts, Rank: "J"},
		},
		Hands: map[string][]Card{
			"p1": {
				{ID: "play-9", Suit: Clubs, Rank: "9"},
				{ID: "hold-k", Suit: Diamonds, Rank: "K"},
			},
			"p2": {{ID: "def", Suit: Spades, Rank: "6"}},
		},
		PlayerOrder: []string{"p1", "p2"},
		Metadata:    map[string]any{"round_limit": 6},
	}

	affordances := ViewAffordances(state, "p1")

	if !affordances.CanAct || !affordances.CanAttack || !affordances.CanPass {
		t.Fatalf("expected attacker affordances to allow attack/pass, got %+v", affordances)
	}
	if affordances.CanDefend || affordances.CanTake || affordances.CanThrowIn || affordances.CanTranslate {
		t.Fatalf("unexpected attacker affordances enabled: %+v", affordances)
	}
	if !slices.Equal(affordances.AttackCardIDs, []string{"play-9"}) {
		t.Fatalf("unexpected attack cards: %v", affordances.AttackCardIDs)
	}
}

func TestViewAffordances_DefenderWindow(t *testing.T) {
	state := GameState{
		MatchID:      "m-aff-defend",
		Status:       StatusPlaying,
		Version:      4,
		Trump:        Spades,
		AttackerID:   "p1",
		DefenderID:   "p2",
		TurnState:    TurnDefend,
		TurnPlayerID: "p2",
		TableCards: []Card{
			{ID: "attack-9", Suit: Hearts, Rank: "9"},
		},
		Hands: map[string][]Card{
			"p1": {{ID: "atk", Suit: Clubs, Rank: "6"}},
			"p2": {
				{ID: "def-j", Suit: Hearts, Rank: "J"},
				{ID: "translate-9", Suit: Clubs, Rank: "9"},
				{ID: "junk", Suit: Diamonds, Rank: "6"},
			},
			"p3": {{ID: "next", Suit: Spades, Rank: "A"}},
		},
		PlayerOrder: []string{"p1", "p2", "p3"},
		Mode:        "perevodnoy",
		Metadata:    map[string]any{"round_limit": 6},
	}

	affordances := ViewAffordances(state, "p2")

	if !affordances.CanAct || !affordances.CanDefend || !affordances.CanTake || !affordances.CanTranslate {
		t.Fatalf("expected defender affordances to allow defend/take/translate, got %+v", affordances)
	}
	if !slices.Equal(affordances.DefendCardIDs, []string{"def-j"}) {
		t.Fatalf("unexpected defend cards: %v", affordances.DefendCardIDs)
	}
	if !slices.Equal(affordances.TranslateCardIDs, []string{"translate-9"}) {
		t.Fatalf("unexpected translate cards: %v", affordances.TranslateCardIDs)
	}
	if !slices.Equal(affordances.DefendableTargetIDs, []string{"attack-9"}) {
		t.Fatalf("unexpected defend targets: %v", affordances.DefendableTargetIDs)
	}
}

func TestViewAffordances_ThrowWindow(t *testing.T) {
	state := GameState{
		MatchID:      "m-aff-throw",
		Status:       StatusPlaying,
		Version:      5,
		Trump:        Hearts,
		AttackerID:   "p1",
		DefenderID:   "p2",
		TurnState:    TurnAttack,
		TurnPlayerID: "p1",
		TableCards: []Card{
			{ID: "a1", Suit: Clubs, Rank: "9"},
			{ID: "d1", Suit: Hearts, Rank: "6"},
		},
		Hands: map[string][]Card{
			"p1": {{ID: "atk", Suit: Clubs, Rank: "6"}},
			"p2": {{ID: "def", Suit: Hearts, Rank: "7"}},
			"p3": {
				{ID: "throw-9", Suit: Diamonds, Rank: "9"},
				{ID: "hold-7", Suit: Spades, Rank: "7"},
			},
		},
		PlayerOrder: []string{"p1", "p2", "p3"},
		Metadata:    map[string]any{"round_limit": 6},
	}

	affordances := ViewAffordances(state, "p3")

	if !affordances.CanAct || !affordances.CanThrowIn {
		t.Fatalf("expected throw-in affordance, got %+v", affordances)
	}
	if !slices.Equal(affordances.ThrowInCardIDs, []string{"throw-9"}) {
		t.Fatalf("unexpected throw-in cards: %v", affordances.ThrowInCardIDs)
	}
}

func TestViewAffordances_ShulerWindow(t *testing.T) {
	now := time.Now().UTC()
	state := GameState{
		MatchID:           "m-aff-shuler",
		Status:            StatusPlaying,
		Version:           6,
		Trump:             Spades,
		AttackerID:        "p1",
		DefenderID:        "p2",
		TurnState:         TurnDefend,
		TurnPlayerID:      "p2",
		TableCards:        []Card{{ID: "attack-10", Suit: Hearts, Rank: "10"}},
		Hands:             map[string][]Card{"p1": {{ID: "atk", Suit: Clubs, Rank: "6"}}, "p2": {{ID: "bad", Suit: Clubs, Rank: "6"}, {ID: "keep", Suit: Diamonds, Rank: "K"}}},
		PlayerOrder:       []string{"p1", "p2"},
		Metadata:          map[string]any{"round_limit": 1},
		ShulerEnabled:     true,
		ShulerPlayerID:    "p2",
		ShulerWindowUntil: now.Add(2 * time.Second),
	}

	shulerAffordances := ViewAffordances(state, "p2")
	if !shulerAffordances.CanShulerPlay {
		t.Fatalf("expected shuler_play affordance, got %+v", shulerAffordances)
	}
	if !slices.Equal(shulerAffordances.ShulerPlayCardIDs, []string{"bad", "keep"}) {
		t.Fatalf("unexpected shuler playable cards: %v", shulerAffordances.ShulerPlayCardIDs)
	}

	reporterAffordances := ViewAffordances(state, "p1")
	if !reporterAffordances.CanShulerReport {
		t.Fatalf("expected shuler_report affordance, got %+v", reporterAffordances)
	}
}
