package games

import (
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

func centsEqual(got, want float64) bool {
	return cents(got) == cents(want)
}

func sumGrossByUser(entries map[string]float64) int64 {
	var total int64
	for _, amount := range entries {
		total += cents(amount)
	}
	return total
}

func sumProfits(entries []PayoutEntry) int64 {
	var total int64
	for _, entry := range entries {
		total += cents(entry.Amount)
	}
	return total
}

func assertSettlementConservation(t *testing.T, pot, commission float64, payouts []PayoutEntry, gross map[string]float64) {
	t.Helper()
	if got, want := sumGrossByUser(gross)+cents(commission), cents(pot); got != want {
		t.Fatalf("gross payouts + commission mismatch: got %d cents want %d cents", got, want)
	}
	if got, want := sumProfits(payouts), -cents(commission); got != want {
		t.Fatalf("profit sum mismatch: got %d cents want %d cents", got, want)
	}
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

	if !centsEqual(pot, 200) {
		t.Fatalf("pot mismatch: got %v want %v", pot, 200.0)
	}
	if !centsEqual(commission, 6) {
		t.Fatalf("commission mismatch: got %v want %v", commission, 6.0)
	}
	if !centsEqual(gross["p1"], 194) {
		t.Fatalf("winner gross mismatch: got %v want %v", gross["p1"], 194.0)
	}
	if !centsEqual(byUser["p1"], 94) {
		t.Fatalf("winner profit mismatch: got %v want %v", byUser["p1"], 94.0)
	}
	if !centsEqual(byUser["p2"], -100) {
		t.Fatalf("loser profit mismatch: got %v want %v", byUser["p2"], -100.0)
	}
	assertSettlementConservation(t, pot, commission, payouts, gross)
}

func TestComputePayouts_ThreePlayersFormula(t *testing.T) {
	state := engine.GameState{
		PlayerOrder:  []string{"p1", "p2", "p3"},
		FinishGroups: [][]string{{"p1"}, {"p2"}, {"p3"}},
	}

	pot, commission, payouts, gross := computePayouts(state, 100, 300)
	byUser := payoutByUser(payouts)

	if !centsEqual(pot, 300) {
		t.Fatalf("pot mismatch: got %v want %v", pot, 300.0)
	}
	if !centsEqual(commission, 9) {
		t.Fatalf("commission mismatch: got %v want %v", commission, 9.0)
	}
	if !centsEqual(gross["p1"], 174.6) {
		t.Fatalf("1st gross mismatch: got %v want %v", gross["p1"], 174.6)
	}
	if !centsEqual(gross["p2"], 116.4) {
		t.Fatalf("2nd gross mismatch: got %v want %v", gross["p2"], 116.4)
	}
	if !centsEqual(gross["p3"], 0) {
		t.Fatalf("3rd gross mismatch: got %v want %v", gross["p3"], 0.0)
	}
	if !centsEqual(byUser["p1"], 74.6) {
		t.Fatalf("1st profit mismatch: got %v want %v", byUser["p1"], 74.6)
	}
	if !centsEqual(byUser["p2"], 16.4) {
		t.Fatalf("2nd profit mismatch: got %v want %v", byUser["p2"], 16.4)
	}
	if !centsEqual(byUser["p3"], -100) {
		t.Fatalf("3rd profit mismatch: got %v want %v", byUser["p3"], -100.0)
	}
	assertSettlementConservation(t, pot, commission, payouts, gross)
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

	if !centsEqual(pot, 400) {
		t.Fatalf("pot mismatch: got %v want %v", pot, 400.0)
	}
	if !centsEqual(commission, 12) {
		t.Fatalf("commission mismatch: got %v want %v", commission, 12.0)
	}
	if !centsEqual(gross["p1"], 155.2) || !centsEqual(gross["p2"], 155.2) {
		t.Fatalf("tie gross mismatch: got p1=%v p2=%v want 155.2", gross["p1"], gross["p2"])
	}
	if !centsEqual(gross["p3"], 77.6) {
		t.Fatalf("3rd gross mismatch: got %v want 77.6", gross["p3"])
	}
	if !centsEqual(gross["p4"], 0) {
		t.Fatalf("4th gross mismatch: got %v want 0", gross["p4"])
	}
	if !centsEqual(byUser["p1"], 55.2) || !centsEqual(byUser["p2"], 55.2) {
		t.Fatalf("tie profit mismatch: got p1=%v p2=%v want 55.2", byUser["p1"], byUser["p2"])
	}
	if !centsEqual(byUser["p3"], -22.4) {
		t.Fatalf("3rd profit mismatch: got %v want -22.4", byUser["p3"])
	}
	if !centsEqual(byUser["p4"], -100) {
		t.Fatalf("4th profit mismatch: got %v want -100", byUser["p4"])
	}
	assertSettlementConservation(t, pot, commission, payouts, gross)
}

func TestComputePayouts_ThreePlayersLowPotTieDeterministicRemainder(t *testing.T) {
	state := engine.GameState{
		PlayerOrder: []string{"p1", "p2", "p3"},
		FinishGroups: [][]string{
			{"p2", "p1"},
			{"p3"},
		},
	}

	pot, commission, payouts, gross := computePayouts(state, 1, 300)
	byUser := payoutByUser(payouts)

	if !centsEqual(pot, 3) {
		t.Fatalf("pot mismatch: got %v want 3.00", pot)
	}
	if !centsEqual(commission, 0.09) {
		t.Fatalf("commission mismatch: got %v want 0.09", commission)
	}
	if !centsEqual(gross["p1"], 1.46) || !centsEqual(gross["p2"], 1.45) {
		t.Fatalf("deterministic tie split mismatch: p1=%v p2=%v", gross["p1"], gross["p2"])
	}
	if !centsEqual(gross["p3"], 0) {
		t.Fatalf("third place gross mismatch: got %v want 0.00", gross["p3"])
	}
	if !centsEqual(byUser["p1"], 0.46) || !centsEqual(byUser["p2"], 0.45) || !centsEqual(byUser["p3"], -1) {
		t.Fatalf("profit mismatch: p1=%v p2=%v p3=%v", byUser["p1"], byUser["p2"], byUser["p3"])
	}
	assertSettlementConservation(t, pot, commission, payouts, gross)
}

func TestComputePayouts_FourPlayersLowPotRemainderHandling(t *testing.T) {
	state := engine.GameState{
		PlayerOrder:  []string{"p1", "p2", "p3", "p4"},
		FinishGroups: [][]string{{"p1"}, {"p2"}, {"p3"}, {"p4"}},
	}

	pot, commission, payouts, gross := computePayouts(state, 0.01, 300)
	byUser := payoutByUser(payouts)

	if !centsEqual(pot, 0.04) {
		t.Fatalf("pot mismatch: got %v want 0.04", pot)
	}
	if !centsEqual(commission, 0) {
		t.Fatalf("commission mismatch: got %v want 0.00", commission)
	}
	if !centsEqual(gross["p1"], 0.02) || !centsEqual(gross["p2"], 0.01) || !centsEqual(gross["p3"], 0.01) {
		t.Fatalf("low-pot allocation mismatch: p1=%v p2=%v p3=%v", gross["p1"], gross["p2"], gross["p3"])
	}
	if !centsEqual(gross["p4"], 0) {
		t.Fatalf("fourth gross mismatch: got %v want 0.00", gross["p4"])
	}
	if !centsEqual(byUser["p1"], 0.01) || !centsEqual(byUser["p2"], 0) || !centsEqual(byUser["p3"], 0) || !centsEqual(byUser["p4"], -0.01) {
		t.Fatalf("profit mismatch: p1=%v p2=%v p3=%v p4=%v", byUser["p1"], byUser["p2"], byUser["p3"], byUser["p4"])
	}
	assertSettlementConservation(t, pot, commission, payouts, gross)
}
