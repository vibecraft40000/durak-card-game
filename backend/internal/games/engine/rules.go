package engine

import (
	"errors"
	"slices"
	"time"
)

var (
	ErrInvalidTurn        = errors.New("invalid turn")
	ErrCardMissing        = errors.New("card not in hand")
	ErrInvalidMove        = errors.New("invalid move")
	ErrCardDoesNotBeat    = errors.New("defense card does not beat attack card")
	ErrAttackCardDenied   = errors.New("attack card rank not allowed")
	ErrTranslateDenied    = errors.New("translate move is not allowed")
	ErrThrowDenied        = errors.New("throw move is not allowed")
	ErrShulerPlayDenied   = errors.New("shuler play is not allowed")
	ErrCannotReportShuler = errors.New("cannot report shuler now")
)

type Action string

const (
	ActionAttack       Action = "attack"
	ActionDefend       Action = "defend"
	ActionThrow        Action = "throw"
	ActionTake         Action = "take"
	ActionPass         Action = "pass"
	ActionTranslate    Action = "translate"
	ActionShulerPlay   Action = "shuler_play"
	ActionShulerReport Action = "shuler_report"
)

const shulerReportWindow = 3 * time.Second

func ApplyAction(state *GameState, playerID string, action Action, cardID string, turnTTL time.Duration) error {
	if state.Status != StatusPlaying {
		return ErrInvalidTurn
	}
	if state.Metadata == nil {
		state.Metadata = map[string]interface{}{}
	}
	if action != ActionShulerReport && action != ActionThrow && state.TurnPlayerID != playerID {
		return ErrInvalidTurn
	}
	resetTurnDeadline := true
	now := time.Now().UTC()

	switch action {
	case ActionShulerPlay:
		if !state.ShulerEnabled || state.ShulerDetected || state.ShulerPlayerID == "" || playerID != state.ShulerPlayerID {
			return ErrShulerPlayDenied
		}
		if state.TurnState != TurnDefend || state.DefenderID != playerID || len(state.TableCards)%2 == 0 || len(state.TableCards) == 0 {
			return ErrShulerPlayDenied
		}
		hand := state.Hands[playerID]
		card, ok := popCard(&hand, cardID)
		if !ok {
			return ErrCardMissing
		}
		state.Hands[playerID] = hand
		state.TableCards = append(state.TableCards, card)
		state.TurnState = TurnAttack
		state.TurnPlayerID = state.AttackerID
		state.ShulerWindowUntil = now.Add(shulerReportWindow)
		state.Metadata["shuler_play_by"] = playerID
	case ActionShulerReport:
		if !state.ShulerEnabled || state.ShulerDetected || state.ShulerPlayerID == "" || playerID == state.ShulerPlayerID {
			return ErrCannotReportShuler
		}
		if state.ShulerWindowUntil.IsZero() || now.After(state.ShulerWindowUntil) {
			return ErrCannotReportShuler
		}
		state.ShulerDetected = true
		state.ShulerWindowUntil = time.Time{}
		state.Metadata["shuler_reported_by"] = playerID
		resetTurnDeadline = false
	case ActionAttack:
		if state.TurnState != TurnAttack || state.AttackerID != playerID {
			return ErrInvalidTurn
		}
		if !canAttackCard(state, cardID) {
			return ErrAttackCardDenied
		}
		if roundAttackCount(state) >= roundLimit(state) {
			return ErrInvalidMove
		}
		hand := state.Hands[playerID]
		card, ok := popCard(&hand, cardID)
		if !ok {
			return ErrCardMissing
		}
		state.Hands[playerID] = hand
		state.TableCards = append(state.TableCards, card)
		state.TurnState = TurnDefend
		state.TurnPlayerID = state.DefenderID
	case ActionDefend:
		if state.TurnState != TurnDefend || state.DefenderID != playerID || len(state.TableCards)%2 == 0 || len(state.TableCards) == 0 {
			return ErrInvalidTurn
		}
		hand := state.Hands[playerID]
		card, ok := popCard(&hand, cardID)
		if !ok {
			return ErrCardMissing
		}
		attackCard := state.TableCards[len(state.TableCards)-1]
		if !beatsForState(state, attackCard, card, playerID) {
			return ErrCardDoesNotBeat
		}
		state.Hands[playerID] = hand
		state.TableCards = append(state.TableCards, card)
		state.TurnState = TurnAttack
		state.TurnPlayerID = state.AttackerID
	case ActionTranslate:
		if state.Mode != "perevodnoy" || state.TurnState != TurnDefend || state.DefenderID != playerID || len(state.TableCards)%2 == 0 || len(state.TableCards) == 0 {
			return ErrTranslateDenied
		}
		hand := state.Hands[playerID]
		card, ok := popCard(&hand, cardID)
		if !ok {
			return ErrCardMissing
		}
		lastAttack := state.TableCards[len(state.TableCards)-1]
		if card.Rank != lastAttack.Rank {
			return ErrTranslateDenied
		}
		nextDefender := nextActivePlayer(state, state.DefenderID, state.AttackerID, state.DefenderID)
		if nextDefender == "" {
			return ErrTranslateDenied
		}
		state.Hands[playerID] = hand
		state.TableCards = append(state.TableCards, card)
		state.DefenderID = nextDefender
		state.TurnState = TurnDefend
		state.TurnPlayerID = nextDefender
		state.Metadata["round_limit"] = len(state.Hands[nextDefender])
	case ActionThrow:
		if state.TurnState != TurnAttack || len(state.TableCards) == 0 || len(state.TableCards)%2 != 0 {
			return ErrThrowDenied
		}
		if playerID == state.DefenderID || playerID == state.AttackerID {
			return ErrThrowDenied
		}
		if !slices.Contains(state.PlayerOrder, playerID) || isEliminated(state, playerID) {
			return ErrThrowDenied
		}
		if !canThrowCard(state, playerID, cardID) {
			return ErrThrowDenied
		}
		if roundAttackCount(state) >= roundLimit(state) {
			return ErrInvalidMove
		}
		hand := state.Hands[playerID]
		card, ok := popCard(&hand, cardID)
		if !ok {
			return ErrCardMissing
		}
		state.Hands[playerID] = hand
		state.TableCards = append(state.TableCards, card)
		state.TurnState = TurnDefend
		state.TurnPlayerID = state.DefenderID
	case ActionTake:
		if state.TurnState != TurnDefend || state.DefenderID != playerID {
			return ErrInvalidTurn
		}
		state.Hands[playerID] = append(state.Hands[playerID], state.TableCards...)
		state.TableCards = nil
		refillHands(state)
		rotateAfterTake(state)
		prepareNextRound(state)
	case ActionPass:
		if state.TurnState != TurnAttack || state.AttackerID != playerID || len(state.TableCards) == 0 || len(state.TableCards)%2 != 0 {
			return ErrInvalidTurn
		}
		state.TableCards = nil
		refillHands(state)
		rotateAfterSuccessfulDefense(state)
		prepareNextRound(state)
	default:
		return ErrInvalidTurn
	}

	maybeFinish(state)
	state.Version++
	if state.Status == StatusPlaying && resetTurnDeadline {
		state.TurnEndsAt = now.Add(turnTTL)
	}
	state.LastActionAt = now
	return nil
}

func rotateAfterSuccessfulDefense(state *GameState) {
	nextAttacker := state.DefenderID
	if isEliminated(state, nextAttacker) {
		nextAttacker = nextActivePlayer(state, nextAttacker)
	}
	nextDefender := nextActivePlayer(state, nextAttacker, nextAttacker)
	if nextAttacker == "" || nextDefender == "" {
		return
	}
	state.AttackerID = nextAttacker
	state.DefenderID = nextDefender
}

func rotateAfterTake(state *GameState) {
	nextAttacker := state.AttackerID
	if isEliminated(state, nextAttacker) {
		nextAttacker = nextActivePlayer(state, nextAttacker)
	}
	nextDefender := nextActivePlayer(state, state.DefenderID, nextAttacker)
	if nextDefender == "" {
		nextDefender = nextActivePlayer(state, nextAttacker, nextAttacker)
	}
	if nextAttacker == "" || nextDefender == "" {
		return
	}
	state.AttackerID = nextAttacker
	state.DefenderID = nextDefender
}

func refillHands(state *GameState) {
	dealCardsInOrderFrom(state.AttackerID, state.PlayerOrder, state.Hands, &state.Deck, 6)
}

func prepareNextRound(state *GameState) {
	state.TurnState = TurnAttack
	state.TurnPlayerID = state.AttackerID
	state.Metadata["round_limit"] = len(state.Hands[state.DefenderID])
}

func maybeFinish(state *GameState) {
	finishedSet := make(map[string]struct{})
	for _, group := range state.FinishGroups {
		for _, playerID := range group {
			finishedSet[playerID] = struct{}{}
		}
	}

	newlyFinished := make([]string, 0)
	for _, playerID := range state.PlayerOrder {
		if _, already := finishedSet[playerID]; already {
			continue
		}
		if isEliminated(state, playerID) {
			newlyFinished = append(newlyFinished, playerID)
		}
	}
	if len(newlyFinished) > 0 {
		state.FinishGroups = append(state.FinishGroups, newlyFinished)
		for _, playerID := range newlyFinished {
			finishedSet[playerID] = struct{}{}
		}
	}

	active := activePlayers(state)
	totalPlayers := len(state.PlayerOrder)
	finishedCount := 0
	for _, group := range state.FinishGroups {
		finishedCount += len(group)
	}

	if finishedCount == totalPlayers {
		finalizeMatchResult(state)
		return
	}

	// The only non-finished player becomes the last place.
	if len(active) == 1 && finishedCount == totalPlayers-1 {
		last := active[0]
		if _, exists := finishedSet[last]; !exists {
			state.FinishGroups = append(state.FinishGroups, []string{last})
		}
		finalizeMatchResult(state)
	}
}

func finalizeMatchResult(state *GameState) {
	if len(state.FinishGroups) == 0 || len(state.FinishGroups[0]) == 0 {
		return
	}
	state.Status = StatusFinished
	state.WinnerPlayer = state.FinishGroups[0][0]
	state.WinnerPlayers = slices.Clone(state.FinishGroups[0])
	state.IsDraw = len(state.WinnerPlayers) > 1
}

func activePlayers(state *GameState) []string {
	out := make([]string, 0, len(state.PlayerOrder))
	for _, playerID := range state.PlayerOrder {
		if !isEliminated(state, playerID) {
			out = append(out, playerID)
		}
	}
	return out
}

func isEliminated(state *GameState, playerID string) bool {
	return len(state.Hands[playerID]) == 0 && len(state.Deck) == 0
}

func nextActivePlayer(state *GameState, current string, excluded ...string) string {
	if len(state.PlayerOrder) == 0 {
		return ""
	}
	excludedSet := make(map[string]struct{}, len(excluded))
	for _, playerID := range excluded {
		excludedSet[playerID] = struct{}{}
	}

	startIndex := -1
	for i, playerID := range state.PlayerOrder {
		if playerID == current {
			startIndex = i
			break
		}
	}
	if startIndex < 0 {
		startIndex = 0
	}
	for step := 1; step <= len(state.PlayerOrder); step++ {
		candidate := state.PlayerOrder[(startIndex+step)%len(state.PlayerOrder)]
		if candidate == "" {
			continue
		}
		if _, blocked := excludedSet[candidate]; blocked {
			continue
		}
		if isEliminated(state, candidate) {
			continue
		}
		return candidate
	}
	return ""
}

// ForceFinishWithWinner ends the match with the given winner (e.g. when opponent abandons).
func ForceFinishWithWinner(state *GameState, winnerID string) {
	state.Status = StatusFinished
	state.WinnerPlayer = winnerID
	state.WinnerPlayers = []string{winnerID}
	state.IsDraw = false
	state.FinishGroups = [][]string{{winnerID}}
	for _, playerID := range state.PlayerOrder {
		if playerID == winnerID {
			continue
		}
		state.FinishGroups = append(state.FinishGroups, []string{playerID})
	}
	state.Version++
}

func popCard(hand *[]Card, cardID string) (Card, bool) {
	for i, card := range *hand {
		if card.ID == cardID {
			(*hand)[i] = (*hand)[len(*hand)-1]
			*hand = (*hand)[:len(*hand)-1]
			return card, true
		}
	}
	return Card{}, false
}

func roundAttackCount(state *GameState) int {
	return (len(state.TableCards) + 1) / 2
}

func roundLimit(state *GameState) int {
	if state.Metadata != nil {
		if raw, ok := state.Metadata["round_limit"]; ok {
			if v, ok := raw.(int); ok && v > 0 {
				return v
			}
			if v, ok := raw.(float64); ok && int(v) > 0 {
				return int(v)
			}
		}
	}
	if v := len(state.Hands[state.DefenderID]); v > 0 {
		return v
	}
	return 1
}

func canAttackCard(state *GameState, cardID string) bool {
	hand := state.Hands[state.AttackerID]
	var selected *Card
	for i := range hand {
		if hand[i].ID == cardID {
			selected = &hand[i]
			break
		}
	}
	if selected == nil {
		return false
	}
	if len(state.TableCards) == 0 {
		return true
	}
	for _, tableCard := range state.TableCards {
		if tableCard.Rank == selected.Rank {
			return true
		}
	}
	return false
}

func canThrowCard(state *GameState, playerID, cardID string) bool {
	hand := state.Hands[playerID]
	var selected *Card
	for i := range hand {
		if hand[i].ID == cardID {
			selected = &hand[i]
			break
		}
	}
	if selected == nil {
		return false
	}
	for _, tableCard := range state.TableCards {
		if tableCard.Rank == selected.Rank {
			return true
		}
	}
	return false
}

func beatsForState(state *GameState, attack Card, defend Card, defenderID string) bool {
	return beats(attack, defend, state.Trump)
}

func beats(attack Card, defend Card, trump Suit) bool {
	if defend.Suit == attack.Suit {
		return rankWeight(defend.Rank) > rankWeight(attack.Rank)
	}
	return defend.Suit == trump && attack.Suit != trump
}

func rankWeight(rank string) int {
	switch rank {
	case "2":
		return 0
	case "3":
		return 1
	case "4":
		return 2
	case "5":
		return 3
	case "6":
		return 4
	case "7":
		return 5
	case "8":
		return 6
	case "9":
		return 7
	case "10":
		return 8
	case "J":
		return 9
	case "Q":
		return 10
	case "K":
		return 11
	case "A":
		return 12
	default:
		return -1
	}
}
