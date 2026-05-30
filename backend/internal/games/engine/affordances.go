package engine

import (
	"slices"
	"time"
)

// PlayerAffordances exposes the server-authoritative actions/cards currently
// available to a specific viewer without changing the mutation protocol.
type PlayerAffordances struct {
	CanAct              bool     `json:"canAct"`
	CanAttack           bool     `json:"canAttack"`
	CanDefend           bool     `json:"canDefend"`
	CanTake             bool     `json:"canTake"`
	CanPass             bool     `json:"canPass"`
	CanThrowIn          bool     `json:"canThrowIn"`
	CanTranslate        bool     `json:"canTranslate"`
	CanShulerPlay       bool     `json:"canShulerPlay"`
	CanShulerReport     bool     `json:"canShulerReport"`
	AttackCardIDs       []string `json:"attackCardIds,omitempty"`
	DefendCardIDs       []string `json:"defendCardIds,omitempty"`
	ThrowInCardIDs      []string `json:"throwInCardIds,omitempty"`
	TranslateCardIDs    []string `json:"translateCardIds,omitempty"`
	ShulerPlayCardIDs   []string `json:"shulerPlayCardIds,omitempty"`
	DefendableTargetIDs []string `json:"defendableTargetCardIds,omitempty"`
}

// ViewAffordances calculates the current server-authoritative affordances for
// the given viewer based on the live engine state.
func ViewAffordances(state GameState, viewerPlayerID string) PlayerAffordances {
	affordances := PlayerAffordances{}
	if viewerPlayerID == "" || state.Status != StatusPlaying {
		return affordances
	}
	if !slices.Contains(state.PlayerOrder, viewerPlayerID) {
		return affordances
	}

	hand := state.Hands[viewerPlayerID]
	canAttackWindow := state.TurnState == TurnAttack && state.TurnPlayerID == viewerPlayerID && state.AttackerID == viewerPlayerID
	canThrowWindow := state.TurnState == TurnAttack &&
		state.TurnPlayerID != viewerPlayerID &&
		state.AttackerID != viewerPlayerID &&
		state.DefenderID != viewerPlayerID &&
		len(state.TableCards) > 0 &&
		len(state.TableCards)%2 == 0 &&
		!isEliminated(&state, viewerPlayerID)
	canDefendWindow := state.TurnState == TurnDefend &&
		state.TurnPlayerID == viewerPlayerID &&
		state.DefenderID == viewerPlayerID &&
		len(state.TableCards) > 0 &&
		len(state.TableCards)%2 == 1

	if canAttackWindow && roundAttackCount(&state) < roundLimit(&state) {
		affordances.AttackCardIDs = collectCardIDs(hand, func(card Card) bool {
			return canAttackCard(&state, card.ID)
		})
		affordances.CanAttack = len(affordances.AttackCardIDs) > 0
	}

	if canThrowWindow && roundAttackCount(&state) < roundLimit(&state) {
		affordances.ThrowInCardIDs = collectCardIDs(hand, func(card Card) bool {
			return canThrowCard(&state, viewerPlayerID, card.ID)
		})
		affordances.CanThrowIn = len(affordances.ThrowInCardIDs) > 0
	}

	if canDefendWindow {
		targetCard := state.TableCards[len(state.TableCards)-1]
		affordances.DefendableTargetIDs = []string{targetCard.ID}
		affordances.DefendCardIDs = collectCardIDs(hand, func(card Card) bool {
			return beatsForState(&state, targetCard, card, viewerPlayerID)
		})
		affordances.CanDefend = len(affordances.DefendCardIDs) > 0
		affordances.CanTake = true

		if state.Mode == "perevodnoy" && nextActivePlayer(&state, state.DefenderID, state.AttackerID, state.DefenderID) != "" {
			affordances.TranslateCardIDs = collectCardIDs(hand, func(card Card) bool {
				return card.Rank == targetCard.Rank
			})
			affordances.CanTranslate = len(affordances.TranslateCardIDs) > 0
		}

		if state.ShulerEnabled && !state.ShulerDetected && state.ShulerPlayerID == viewerPlayerID {
			affordances.ShulerPlayCardIDs = collectCardIDs(hand, func(card Card) bool {
				return card.ID != ""
			})
			affordances.CanShulerPlay = len(affordances.ShulerPlayCardIDs) > 0
		}
	}

	affordances.CanPass = state.TurnState == TurnAttack &&
		state.TurnPlayerID == viewerPlayerID &&
		state.AttackerID == viewerPlayerID &&
		len(state.TableCards) > 0 &&
		len(state.TableCards)%2 == 0

	affordances.CanShulerReport = canReportShuler(&state, viewerPlayerID, time.Now().UTC())
	affordances.CanAct =
		affordances.CanAttack ||
			affordances.CanDefend ||
			affordances.CanTake ||
			affordances.CanPass ||
			affordances.CanThrowIn ||
			affordances.CanTranslate ||
			affordances.CanShulerPlay ||
			affordances.CanShulerReport

	return affordances
}

func collectCardIDs(cards []Card, allow func(card Card) bool) []string {
	if len(cards) == 0 {
		return nil
	}
	out := make([]string, 0, len(cards))
	for _, card := range cards {
		if !allow(card) {
			continue
		}
		out = append(out, card.ID)
	}
	return out
}

func canReportShuler(state *GameState, playerID string, now time.Time) bool {
	if state == nil || playerID == "" || state.Status != StatusPlaying {
		return false
	}
	if !state.ShulerEnabled || state.ShulerDetected || state.ShulerPlayerID == "" {
		return false
	}
	if playerID == state.ShulerPlayerID {
		return false
	}
	if state.ShulerWindowUntil.IsZero() || now.After(state.ShulerWindowUntil) {
		return false
	}
	return slices.Contains(state.PlayerOrder, playerID)
}
