package engine

import (
	"math/rand"
	"slices"
	"time"

	"github.com/google/uuid"
)

type Suit string

const (
	Hearts   Suit = "hearts"
	Diamonds Suit = "diamonds"
	Clubs    Suit = "clubs"
	Spades   Suit = "spades"
)

type Card struct {
	ID   string `json:"id"`
	Suit Suit   `json:"suit"`
	Rank string `json:"rank"`
}

type PlayerState struct {
	ID    string `json:"id"`
	Hand  []Card `json:"hand"`
	Alive bool   `json:"alive"`
}

type TurnState string

const (
	TurnAttack TurnState = "attack"
	TurnDefend TurnState = "defend"
)

type GameStatus string

const (
	StatusWaiting  GameStatus = "waiting"
	StatusPlaying  GameStatus = "playing"
	StatusFinished GameStatus = "finished"
)

type GameState struct {
	MatchID      string                 `json:"match_id"`
	Status       GameStatus             `json:"status"`
	Deck         []Card                 `json:"deck"`
	Trump        Suit                   `json:"trump"`
	AttackerID   string                 `json:"attacker"`
	DefenderID   string                 `json:"defender"`
	TableCards   []Card                 `json:"table_cards"`
	Hands        map[string][]Card      `json:"hands"`
	TurnState    TurnState              `json:"turn_state"`
	TurnPlayerID string                 `json:"turn_player_id"`
	TurnEndsAt   time.Time              `json:"turn_ends_at"`
	WinnerPlayer string                 `json:"winner_player_id,omitempty"`
	PlayerOrder  []string               `json:"player_order"`
	LastActionAt time.Time              `json:"last_action_at"`
	StartedAt    time.Time              `json:"started_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

var ranks = []string{"6", "7", "8", "9", "10", "J", "Q", "K", "A"}
var suits = []Suit{Hearts, Diamonds, Clubs, Spades}

func NewGameState(matchID string, players []string, turnTTL time.Duration) GameState {
	deck := buildDeck()
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	hands := make(map[string][]Card, len(players))
	for _, playerID := range players {
		hands[playerID] = []Card{}
	}

	dealCardsInOrder(players, hands, &deck, 6)
	attacker := players[0]
	defender := players[1]
	trump := deck[len(deck)-1].Suit

	now := time.Now().UTC()
	return GameState{
		MatchID:      matchID,
		Status:       StatusPlaying,
		Deck:         deck,
		StartedAt:    now,
		Trump:        trump,
		AttackerID:   attacker,
		DefenderID:   defender,
		TableCards:   []Card{},
		Hands:        hands,
		TurnState:    TurnAttack,
		TurnPlayerID: attacker,
		TurnEndsAt:   time.Now().Add(turnTTL).UTC(),
		PlayerOrder:  slices.Clone(players),
		LastActionAt: now,
		Metadata: map[string]interface{}{
			"round_limit": len(hands[defender]),
		},
	}
}

func buildDeck() []Card {
	deck := make([]Card, 0, len(ranks)*len(suits))
	for _, suit := range suits {
		for _, rank := range ranks {
			deck = append(deck, Card{
				ID:   uuid.NewString(),
				Suit: suit,
				Rank: rank,
			})
		}
	}
	return deck
}

func dealCardsInOrder(playerOrder []string, hands map[string][]Card, deck *[]Card, target int) {
	for _, playerID := range playerOrder {
		hand := hands[playerID]
		for len(hand) < target && len(*deck) > 0 {
			last := len(*deck) - 1
			hand = append(hand, (*deck)[last])
			*deck = (*deck)[:last]
		}
		hands[playerID] = hand
	}
}
