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

func TestApplyAction_ShulerPlayAndReportWindow(t *testing.T) {
	baseState := func() GameState {
		return GameState{
			MatchID:      "m3",
			Status:       StatusPlaying,
			Version:      1,
			Trump:        Spades,
			AttackerID:   "p1",
			DefenderID:   "p2",
			TurnState:    TurnDefend,
			TurnPlayerID: "p2",
			TableCards:   []Card{{ID: "attack", Suit: Hearts, Rank: "10"}},
			Hands: map[string][]Card{
				"p1": {{ID: "safe", Suit: Hearts, Rank: "J"}, {ID: "attack-next", Suit: Clubs, Rank: "8"}},
				"p2": {{ID: "bad", Suit: Clubs, Rank: "6"}, {ID: "keep", Suit: Diamonds, Rank: "K"}},
			},
			PlayerOrder:    []string{"p1", "p2"},
			Metadata:       map[string]interface{}{"round_limit": 1},
			ShulerEnabled:  true,
			ShulerPlayerID: "p2",
			TurnEndsAt:     time.Now().UTC().Add(30 * time.Second),
		}
	}

	stateAllowed := baseState()
	if err := ApplyAction(&stateAllowed, "p2", ActionShulerPlay, "bad", 30*time.Second); err != nil {
		t.Fatalf("shuler play should pass, got: %v", err)
	}
	if stateAllowed.ShulerWindowUntil.IsZero() {
		t.Fatalf("expected report window to be opened by shuler play")
	}
	if stateAllowed.TurnState != TurnAttack || stateAllowed.TurnPlayerID != "p1" {
		t.Fatalf("expected turn to move to attacker after shuler play, got state=%s player=%s", stateAllowed.TurnState, stateAllowed.TurnPlayerID)
	}

	stateBlocked := baseState()
	if err := ApplyAction(&stateBlocked, "p2", ActionShulerPlay, "bad", 30*time.Second); err != nil {
		t.Fatalf("shuler play should pass before report, got: %v", err)
	}
	prevDeadline := stateBlocked.TurnEndsAt
	if err := ApplyAction(&stateBlocked, "p1", ActionShulerReport, "", 30*time.Second); err != nil {
		t.Fatalf("shuler report failed: %v", err)
	}
	if !stateBlocked.ShulerDetected {
		t.Fatalf("expected shuler to be detected after report")
	}
	if !stateBlocked.TurnEndsAt.Equal(prevDeadline) {
		t.Fatalf("report should not reset turn deadline")
	}
	if err := ApplyAction(&stateBlocked, "p1", ActionPass, "", 30*time.Second); err != nil {
		t.Fatalf("attacker pass after report should pass, got: %v", err)
	}
	if err := ApplyAction(&stateBlocked, "p2", ActionShulerPlay, "bad", 30*time.Second); err != ErrShulerPlayDenied {
		t.Fatalf("expected ErrShulerPlayDenied after report, got: %v", err)
	}
}

func TestApplyAction_ShulerReportOutsideWindowDenied(t *testing.T) {
	state := GameState{
		MatchID:      "m-shuler-expired",
		Status:       StatusPlaying,
		Version:      1,
		Trump:        Spades,
		AttackerID:   "p1",
		DefenderID:   "p2",
		TurnState:    TurnAttack,
		TurnPlayerID: "p1",
		TableCards:   []Card{{ID: "attack", Suit: Hearts, Rank: "10"}, {ID: "bad", Suit: Clubs, Rank: "6"}},
		Hands: map[string][]Card{
			"p1": {},
			"p2": {},
		},
		PlayerOrder:       []string{"p1", "p2"},
		Metadata:          map[string]interface{}{"round_limit": 1},
		ShulerEnabled:     true,
		ShulerPlayerID:    "p2",
		ShulerWindowUntil: time.Now().UTC().Add(-1 * time.Second),
		TurnEndsAt:        time.Now().UTC().Add(20 * time.Second),
	}

	if err := ApplyAction(&state, "p1", ActionShulerReport, "", 30*time.Second); err != ErrCannotReportShuler {
		t.Fatalf("expected ErrCannotReportShuler outside window, got: %v", err)
	}
}

func TestApplyAction_ThrowInByThirdPlayer(t *testing.T) {
	state := NewGameStateDeterministicWithConfig(
		"m-throw",
		[]string{"p1", "p2", "p3"},
		30*time.Second,
		GameConfig{DeckSize: 36, Mode: "podkidnoy"},
		88,
	)

	state.AttackerID = "p1"
	state.DefenderID = "p2"
	state.TurnState = TurnAttack
	state.TurnPlayerID = "p1"
	state.TableCards = []Card{
		{ID: "a1", Suit: Hearts, Rank: "9"},
		{ID: "d1", Suit: Hearts, Rank: "J"},
	}
	state.Hands["p2"] = []Card{{ID: "def-q", Suit: Hearts, Rank: "Q"}}
	state.Hands["p3"] = []Card{{ID: "throw9", Suit: Clubs, Rank: "9"}, {ID: "keep", Suit: Diamonds, Rank: "6"}}

	if err := ApplyAction(&state, "p3", ActionThrow, "throw9", 30*time.Second); err != nil {
		t.Fatalf("throw-in failed: %v", err)
	}
	if state.TurnState != TurnDefend || state.TurnPlayerID != "p2" {
		t.Fatalf("defender should keep turn after throw-in, state=%s player=%s", state.TurnState, state.TurnPlayerID)
	}
	if len(state.TableCards) != 3 {
		t.Fatalf("expected 3 cards on table after throw-in, got %d", len(state.TableCards))
	}

	if err := ApplyAction(&state, "p2", ActionDefend, "def-q", 30*time.Second); err != nil {
		t.Fatalf("defender should be able to defend after throw-in, got %v", err)
	}
	if state.TurnState != TurnAttack || state.TurnPlayerID != "p1" {
		t.Fatalf("expected turn to return to attacker after defense, state=%s player=%s", state.TurnState, state.TurnPlayerID)
	}
}

func TestApplyAction_ThrowInDeniedForDefender(t *testing.T) {
	state := NewGameStateDeterministicWithConfig(
		"m-throw-denied",
		[]string{"p1", "p2", "p3"},
		30*time.Second,
		GameConfig{DeckSize: 36, Mode: "podkidnoy"},
		99,
	)

	state.AttackerID = "p1"
	state.DefenderID = "p2"
	state.TurnState = TurnAttack
	state.TurnPlayerID = "p1"
	state.TableCards = []Card{
		{ID: "a1", Suit: Hearts, Rank: "7"},
		{ID: "d1", Suit: Hearts, Rank: "8"},
	}
	state.Hands["p2"] = []Card{{ID: "def-throw", Suit: Clubs, Rank: "7"}}

	if err := ApplyAction(&state, "p2", ActionThrow, "def-throw", 30*time.Second); err != ErrThrowDenied {
		t.Fatalf("expected ErrThrowDenied for defender throw-in, got %v", err)
	}
}

func TestApplyAction_ThrowInDeniedDuringDefendPhase(t *testing.T) {
	state := NewGameStateDeterministicWithConfig(
		"m-throw-defend-phase",
		[]string{"p1", "p2", "p3"},
		30*time.Second,
		GameConfig{DeckSize: 36, Mode: "podkidnoy"},
		100,
	)

	state.AttackerID = "p1"
	state.DefenderID = "p2"
	state.TurnState = TurnDefend
	state.TurnPlayerID = "p2"
	state.TableCards = []Card{{ID: "a1", Suit: Hearts, Rank: "7"}}
	state.Hands["p3"] = []Card{{ID: "throw7", Suit: Clubs, Rank: "7"}}

	if err := ApplyAction(&state, "p3", ActionThrow, "throw7", 30*time.Second); err != ErrThrowDenied {
		t.Fatalf("expected ErrThrowDenied in defend phase, got %v", err)
	}
}

func TestApplyAction_ThrowInResetsTurnDeadline(t *testing.T) {
	state := NewGameStateDeterministicWithConfig(
		"m-throw-deadline",
		[]string{"p1", "p2", "p3"},
		30*time.Second,
		GameConfig{DeckSize: 36, Mode: "podkidnoy"},
		101,
	)

	state.AttackerID = "p1"
	state.DefenderID = "p2"
	state.TurnState = TurnAttack
	state.TurnPlayerID = "p1"
	state.TableCards = []Card{
		{ID: "a1", Suit: Hearts, Rank: "8"},
		{ID: "d1", Suit: Hearts, Rank: "9"},
	}
	state.Hands["p3"] = []Card{{ID: "throw8", Suit: Clubs, Rank: "8"}}
	state.TurnEndsAt = time.Now().UTC().Add(1 * time.Second)
	prevDeadline := state.TurnEndsAt

	if err := ApplyAction(&state, "p3", ActionThrow, "throw8", 30*time.Second); err != nil {
		t.Fatalf("throw-in failed: %v", err)
	}
	if !state.TurnEndsAt.After(prevDeadline) {
		t.Fatalf("expected throw-in to reset turn deadline, prev=%s new=%s", prevDeadline, state.TurnEndsAt)
	}
}

func TestApplyAction_ThrowInByTwoPlayersOrder4Players(t *testing.T) {
	state := GameState{
		MatchID:      "m-throw-order-4p",
		Status:       StatusPlaying,
		Version:      1,
		Trump:        Hearts,
		AttackerID:   "p1",
		DefenderID:   "p2",
		TurnState:    TurnAttack,
		TurnPlayerID: "p1",
		TableCards: []Card{
			{ID: "a1", Suit: Clubs, Rank: "9"},
			{ID: "d1", Suit: Hearts, Rank: "6"},
		},
		Deck: []Card{
			{ID: "stock-1", Suit: Clubs, Rank: "A"},
		},
		Hands: map[string][]Card{
			"p1": {{ID: "keep1", Suit: Clubs, Rank: "6"}},
			"p2": {{ID: "d2", Suit: Hearts, Rank: "7"}, {ID: "d3", Suit: Hearts, Rank: "8"}, {ID: "keep2", Suit: Spades, Rank: "6"}},
			"p3": {{ID: "t3", Suit: Diamonds, Rank: "9"}, {ID: "keep3", Suit: Clubs, Rank: "7"}},
			"p4": {{ID: "t4", Suit: Spades, Rank: "9"}, {ID: "keep4", Suit: Diamonds, Rank: "7"}},
		},
		PlayerOrder: []string{"p1", "p2", "p3", "p4"},
		Metadata:    map[string]interface{}{"round_limit": 6},
		TurnEndsAt:  time.Now().UTC().Add(20 * time.Second),
	}

	if err := ApplyAction(&state, "p3", ActionThrow, "t3", 30*time.Second); err != nil {
		t.Fatalf("first throw-in failed: %v", err)
	}
	if state.TurnState != TurnDefend || state.TurnPlayerID != "p2" {
		t.Fatalf("expected defender turn after first throw-in, state=%s player=%s", state.TurnState, state.TurnPlayerID)
	}
	if err := ApplyAction(&state, "p2", ActionDefend, "d2", 30*time.Second); err != nil {
		t.Fatalf("defender should beat first throw-in, got %v", err)
	}
	if state.TurnState != TurnAttack || state.TurnPlayerID != "p1" {
		t.Fatalf("expected attacker turn after first defense, state=%s player=%s", state.TurnState, state.TurnPlayerID)
	}

	if err := ApplyAction(&state, "p4", ActionThrow, "t4", 30*time.Second); err != nil {
		t.Fatalf("second throw-in failed: %v", err)
	}
	if err := ApplyAction(&state, "p2", ActionDefend, "d3", 30*time.Second); err != nil {
		t.Fatalf("defender should beat second throw-in, got %v", err)
	}
	if len(state.TableCards) != 6 {
		t.Fatalf("expected 6 cards on table after two throw-ins, got %d", len(state.TableCards))
	}

	if err := ApplyAction(&state, "p1", ActionPass, "", 30*time.Second); err != nil {
		t.Fatalf("pass after defended throw-ins failed: %v", err)
	}
	if len(state.TableCards) != 0 {
		t.Fatalf("table should be empty after pass, got %d cards", len(state.TableCards))
	}
	if state.AttackerID != "p2" || state.DefenderID != "p3" {
		t.Fatalf("rotation after successful defense mismatch, attacker=%s defender=%s", state.AttackerID, state.DefenderID)
	}
}

func TestApplyAction_ThrowInRoundLimitAcrossMultipleThrowers(t *testing.T) {
	state := GameState{
		MatchID:      "m-throw-limit-4p",
		Status:       StatusPlaying,
		Version:      1,
		Trump:        Hearts,
		AttackerID:   "p1",
		DefenderID:   "p2",
		TurnState:    TurnAttack,
		TurnPlayerID: "p1",
		TableCards: []Card{
			{ID: "a1", Suit: Clubs, Rank: "9"},
			{ID: "d1", Suit: Hearts, Rank: "6"},
		},
		Deck: []Card{
			{ID: "stock-1", Suit: Clubs, Rank: "A"},
		},
		Hands: map[string][]Card{
			"p1": {},
			"p2": {{ID: "d2", Suit: Hearts, Rank: "7"}},
			"p3": {{ID: "t3", Suit: Diamonds, Rank: "9"}},
			"p4": {{ID: "t4", Suit: Spades, Rank: "9"}},
		},
		PlayerOrder: []string{"p1", "p2", "p3", "p4"},
		Metadata:    map[string]interface{}{"round_limit": 2},
		TurnEndsAt:  time.Now().UTC().Add(20 * time.Second),
	}

	if err := ApplyAction(&state, "p3", ActionThrow, "t3", 30*time.Second); err != nil {
		t.Fatalf("first throw-in failed: %v", err)
	}
	if err := ApplyAction(&state, "p2", ActionDefend, "d2", 30*time.Second); err != nil {
		t.Fatalf("defender should beat first throw-in, got %v", err)
	}
	if err := ApplyAction(&state, "p4", ActionThrow, "t4", 30*time.Second); err != ErrInvalidMove {
		t.Fatalf("expected ErrInvalidMove when throw exceeds round limit, got %v", err)
	}
	if len(state.TableCards) != 4 {
		t.Fatalf("table should stay unchanged when throw is denied, got %d cards", len(state.TableCards))
	}
	if len(state.Hands["p4"]) != 1 {
		t.Fatalf("thrower hand should not change on denied throw-in")
	}
}

func TestApplyAction_TakeAfterThrowInRotatesFourPlayers(t *testing.T) {
	state := GameState{
		MatchID:      "m-throw-take-4p",
		Status:       StatusPlaying,
		Version:      1,
		Trump:        Hearts,
		AttackerID:   "p1",
		DefenderID:   "p2",
		TurnState:    TurnAttack,
		TurnPlayerID: "p1",
		Deck: []Card{
			{ID: "stock-1", Suit: Diamonds, Rank: "A"},
		},
		TableCards: []Card{
			{ID: "a1", Suit: Clubs, Rank: "9"},
			{ID: "d1", Suit: Hearts, Rank: "6"},
		},
		Hands: map[string][]Card{
			"p1": {},
			"p2": {{ID: "hold", Suit: Diamonds, Rank: "6"}},
			"p3": {{ID: "t3", Suit: Spades, Rank: "9"}, {ID: "keep3", Suit: Clubs, Rank: "6"}},
			"p4": {},
		},
		PlayerOrder: []string{"p1", "p2", "p3", "p4"},
		Metadata:    map[string]interface{}{"round_limit": 6},
		TurnEndsAt:  time.Now().UTC().Add(20 * time.Second),
	}

	if err := ApplyAction(&state, "p3", ActionThrow, "t3", 30*time.Second); err != nil {
		t.Fatalf("throw-in failed: %v", err)
	}
	if err := ApplyAction(&state, "p2", ActionTake, "", 30*time.Second); err != nil {
		t.Fatalf("take after throw-in failed: %v", err)
	}
	if len(state.TableCards) != 0 {
		t.Fatalf("table should be empty after take, got %d cards", len(state.TableCards))
	}
	if len(state.Hands["p2"]) != 4 {
		t.Fatalf("defender should collect all table cards; expected hand size 4, got %d", len(state.Hands["p2"]))
	}
	if state.AttackerID != "p1" || state.DefenderID != "p3" {
		t.Fatalf("rotation after take mismatch, attacker=%s defender=%s", state.AttackerID, state.DefenderID)
	}
	if state.TurnState != TurnAttack || state.TurnPlayerID != "p1" {
		t.Fatalf("next round should start from attacker, state=%s player=%s", state.TurnState, state.TurnPlayerID)
	}
}

func TestApplyAction_PassCreatesSimultaneousFinishGroup(t *testing.T) {
	state := GameState{
		MatchID:      "m-finish-group-pass",
		Status:       StatusPlaying,
		Version:      1,
		Trump:        Hearts,
		AttackerID:   "p1",
		DefenderID:   "p2",
		TurnState:    TurnAttack,
		TurnPlayerID: "p1",
		Deck:         []Card{},
		TableCards: []Card{
			{ID: "a1", Suit: Clubs, Rank: "9"},
			{ID: "d1", Suit: Hearts, Rank: "6"},
		},
		Hands: map[string][]Card{
			"p1": {},
			"p2": {},
			"p3": {{ID: "keep3", Suit: Spades, Rank: "A"}},
			"p4": {{ID: "keep4", Suit: Diamonds, Rank: "A"}},
		},
		PlayerOrder: []string{"p1", "p2", "p3", "p4"},
		Metadata:    map[string]interface{}{"round_limit": 1},
		TurnEndsAt:  time.Now().UTC().Add(20 * time.Second),
	}

	if err := ApplyAction(&state, "p1", ActionPass, "", 30*time.Second); err != nil {
		t.Fatalf("pass with simultaneous finish should succeed: %v", err)
	}
	if state.Status != StatusPlaying {
		t.Fatalf("match should still be active; got status=%s", state.Status)
	}
	if len(state.FinishGroups) != 1 {
		t.Fatalf("expected one finish group, got %d", len(state.FinishGroups))
	}
	group := state.FinishGroups[0]
	if len(group) != 2 || group[0] != "p1" || group[1] != "p2" {
		t.Fatalf("unexpected simultaneous finish group: %v", group)
	}
	if state.AttackerID != "p3" || state.DefenderID != "p4" {
		t.Fatalf("rotation after finishing pair mismatch, attacker=%s defender=%s", state.AttackerID, state.DefenderID)
	}
}

func TestMaybeFinish_AppendsFinalSimultaneousGroupAndKeepsWinnerGroup(t *testing.T) {
	state := GameState{
		MatchID: "m-finish-groups-finalize",
		Status:  StatusPlaying,
		Deck:    []Card{},
		PlayerOrder: []string{
			"p1", "p2", "p3", "p4",
		},
		Hands: map[string][]Card{
			"p1": {},
			"p2": {},
			"p3": {},
			"p4": {},
		},
		FinishGroups: [][]string{
			{"p1", "p2"},
		},
	}

	maybeFinish(&state)

	if state.Status != StatusFinished {
		t.Fatalf("expected finished status, got %s", state.Status)
	}
	if len(state.FinishGroups) != 2 {
		t.Fatalf("expected two finish groups, got %d", len(state.FinishGroups))
	}
	lastGroup := state.FinishGroups[1]
	if len(lastGroup) != 2 || lastGroup[0] != "p3" || lastGroup[1] != "p4" {
		t.Fatalf("unexpected final simultaneous group: %v", lastGroup)
	}
	if len(state.WinnerPlayers) != 2 || state.WinnerPlayers[0] != "p1" || state.WinnerPlayers[1] != "p2" {
		t.Fatalf("winner group should remain first finish group, got %v", state.WinnerPlayers)
	}
	if !state.IsDraw {
		t.Fatalf("expected draw=true for simultaneous winners")
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
