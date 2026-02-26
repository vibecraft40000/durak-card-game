package games

import (
	"math"
	"testing"

	"durakonline/backend/internal/games/engine"
)

func payoutByUser(entries []PayoutEntry) map[string]float64 {
	out := make(map[string]float64, len(entries))
	for _, entry := range entries {
		out[entry.UserID] = entry.Amount
	}
	return out
}

func almostEqual(got, want float64) bool {
	return math.Abs(got-want) < 0.0001
}

func TestComputePayouts_TwoPlayers(t *testing.T) {
	state := engine.GameState{
		PlayerOrder:   []string{"p1", "p2"},
		FinishGroups:  [][]string{{"p1"}, {"p2"}},
		WinnerPlayer:  "p1",
		WinnerPlayers: []string{"p1"},
	}

	pot, commission, payouts, gross := computePayouts(state, 100, 300)
	byUser := payoutByUser(payouts)

	if !almostEqual(pot, 200) {
		t.Fatalf("pot mismatch: got %v want %v", pot, 200.0)
	}
	if !almostEqual(commission, 6) {
		t.Fatalf("commission mismatch: got %v want %v", commission, 6.0)
	}
	if !almostEqual(gross["p1"], 194) {
		t.Fatalf("winner gross mismatch: got %v want %v", gross["p1"], 194.0)
	}
	if !almostEqual(byUser["p1"], 94) {
		t.Fatalf("winner profit mismatch: got %v want %v", byUser["p1"], 94.0)
	}
	if !almostEqual(byUser["p2"], -100) {
		t.Fatalf("loser profit mismatch: got %v want %v", byUser["p2"], -100.0)
	}
}

func TestComputePayouts_ThreePlayersFormula(t *testing.T) {
	state := engine.GameState{
		PlayerOrder:  []string{"p1", "p2", "p3"},
		FinishGroups: [][]string{{"p1"}, {"p2"}, {"p3"}},
	}

	pot, commission, payouts, gross := computePayouts(state, 100, 300)
	byUser := payoutByUser(payouts)

	if !almostEqual(pot, 300) {
		t.Fatalf("pot mismatch: got %v want %v", pot, 300.0)
	}
	if !almostEqual(commission, 9) {
		t.Fatalf("commission mismatch: got %v want %v", commission, 9.0)
	}
	if !almostEqual(gross["p1"], 174.6) {
		t.Fatalf("1st gross mismatch: got %v want %v", gross["p1"], 174.6)
	}
	if !almostEqual(gross["p2"], 116.4) {
		t.Fatalf("2nd gross mismatch: got %v want %v", gross["p2"], 116.4)
	}
	if !almostEqual(gross["p3"], 0) {
		t.Fatalf("3rd gross mismatch: got %v want %v", gross["p3"], 0.0)
	}
	if !almostEqual(byUser["p1"], 74.6) {
		t.Fatalf("1st profit mismatch: got %v want %v", byUser["p1"], 74.6)
	}
	if !almostEqual(byUser["p2"], 16.4) {
		t.Fatalf("2nd profit mismatch: got %v want %v", byUser["p2"], 16.4)
	}
	if !almostEqual(byUser["p3"], -100) {
		t.Fatalf("3rd profit mismatch: got %v want %v", byUser["p3"], -100.0)
	}
}

func TestComputePayouts_FourPlayersWithTieForFirst(t *testing.T) {
	state := engine.GameState{
		PlayerOrder: []string{"p1", "p2", "p3", "p4"},
		FinishGroups: [][]string{
			{"p1", "p2"},
			{"p3"},
			{"p4"},
		},
	}

	pot, commission, payouts, gross := computePayouts(state, 100, 300)
	byUser := payoutByUser(payouts)

	if !almostEqual(pot, 400) {
		t.Fatalf("pot mismatch: got %v want %v", pot, 400.0)
	}
	if !almostEqual(commission, 12) {
		t.Fatalf("commission mismatch: got %v want %v", commission, 12.0)
	}
	if !almostEqual(gross["p1"], 155.2) || !almostEqual(gross["p2"], 155.2) {
		t.Fatalf("tie gross mismatch: got p1=%v p2=%v want 155.2", gross["p1"], gross["p2"])
	}
	if !almostEqual(gross["p3"], 77.6) {
		t.Fatalf("3rd gross mismatch: got %v want 77.6", gross["p3"])
	}
	if !almostEqual(gross["p4"], 0) {
		t.Fatalf("4th gross mismatch: got %v want 0", gross["p4"])
	}
	if !almostEqual(byUser["p1"], 55.2) || !almostEqual(byUser["p2"], 55.2) {
		t.Fatalf("tie profit mismatch: got p1=%v p2=%v want 55.2", byUser["p1"], byUser["p2"])
	}
	if !almostEqual(byUser["p3"], -22.4) {
		t.Fatalf("3rd profit mismatch: got %v want -22.4", byUser["p3"])
	}
	if !almostEqual(byUser["p4"], -100) {
		t.Fatalf("4th profit mismatch: got %v want -100", byUser["p4"])
	}
}
