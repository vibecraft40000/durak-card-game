package engine

import (
	"errors"
	"time"
)

var (
	ErrInvalidTurn      = errors.New("invalid turn")
	ErrCardMissing      = errors.New("card not in hand")
	ErrInvalidMove      = errors.New("invalid move")
	ErrCardDoesNotBeat  = errors.New("defense card does not beat attack card")
	ErrAttackCardDenied = errors.New("attack card rank not allowed")
)

type Action string

const (
	ActionAttack Action = "attack"
	ActionDefend Action = "defend"
	ActionTake   Action = "take"
	ActionPass   Action = "pass"
)

func ApplyAction(state *GameState, playerID string, action Action, cardID string, turnTTL time.Duration) error {
	if state.Status != StatusPlaying {
		return ErrInvalidTurn
	}
	if state.Metadata == nil {
		state.Metadata = map[string]interface{}{}
	}
	if state.TurnPlayerID != playerID {
		return ErrInvalidTurn
	}

	switch action {
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
		if !beats(attackCard, card, state.Trump) {
			return ErrCardDoesNotBeat
		}
		state.Hands[playerID] = hand
		state.TableCards = append(state.TableCards, card)
		state.TurnState = TurnAttack
		state.TurnPlayerID = state.AttackerID
	case ActionTake:
		if state.TurnState != TurnDefend || state.DefenderID != playerID {
			return ErrInvalidTurn
		}
		state.Hands[playerID] = append(state.Hands[playerID], state.TableCards...)
		state.TableCards = nil
		swapRoles(state)
		refillHands(state)
		state.TurnState = TurnAttack
		state.TurnPlayerID = state.AttackerID
		state.Metadata["round_limit"] = len(state.Hands[state.DefenderID])
	case ActionPass:
		if state.TurnState != TurnAttack || state.AttackerID != playerID || len(state.TableCards) == 0 || len(state.TableCards)%2 != 0 {
			return ErrInvalidTurn
		}
		state.TableCards = nil
		refillHands(state)
		swapRoles(state)
		state.TurnState = TurnAttack
		state.TurnPlayerID = state.AttackerID
		state.Metadata["round_limit"] = len(state.Hands[state.DefenderID])
	default:
		return ErrInvalidTurn
	}

	maybeFinish(state)
	state.Version++
	state.TurnEndsAt = time.Now().Add(turnTTL).UTC()
	state.LastActionAt = time.Now().UTC()
	return nil
}

func swapRoles(state *GameState) {
	state.AttackerID, state.DefenderID = state.DefenderID, state.AttackerID
}

func refillHands(state *GameState) {
	dealCardsInOrder(state.PlayerOrder, state.Hands, &state.Deck, 6)
}

func maybeFinish(state *GameState) {
	for playerID, hand := range state.Hands {
		if len(hand) == 0 && len(state.Deck) == 0 {
			state.Status = StatusFinished
			state.WinnerPlayer = playerID
			return
		}
	}
}

// ForceFinishWithWinner ends the match with the given winner (e.g. when opponent abandons).
func ForceFinishWithWinner(state *GameState, winnerID string) {
	state.Status = StatusFinished
	state.WinnerPlayer = winnerID
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
	return len(state.Hands[state.DefenderID])
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

func beats(attack Card, defend Card, trump Suit) bool {
	if defend.Suit == attack.Suit {
		return rankWeight(defend.Rank) > rankWeight(attack.Rank)
	}
	return defend.Suit == trump && attack.Suit != trump
}

func rankWeight(rank string) int {
	switch rank {
	case "6":
		return 0
	case "7":
		return 1
	case "8":
		return 2
	case "9":
		return 3
	case "10":
		return 4
	case "J":
		return 5
	case "Q":
		return 6
	case "K":
		return 7
	case "A":
		return 8
	default:
		return -1
	}
}
