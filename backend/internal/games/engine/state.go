package engine

import (
	"math/rand"
	"slices"
	"strings"
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
	Version      int64                  `json:"version"`
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
	WinnerPlayers []string              `json:"winner_player_ids,omitempty"`
	IsDraw       bool                   `json:"is_draw,omitempty"`
	FinishGroups [][]string             `json:"finish_groups,omitempty"`
	PlayerOrder  []string               `json:"player_order"`
	Mode         string                 `json:"mode,omitempty"`
	DeckType     int                    `json:"deck_type,omitempty"`
	ShulerEnabled bool                  `json:"shuler_enabled,omitempty"`
	ShulerPlayerID string               `json:"shuler_player_id,omitempty"`
	ShulerDetected bool                 `json:"shuler_detected,omitempty"`
	LastActionAt time.Time              `json:"last_action_at"`
	StartedAt    time.Time              `json:"started_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type GameConfig struct {
	DeckSize      int
	Mode          string
	ShulerEnabled bool
	ShulerPlayerID string
}

var rankOrder = []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
var suits = []Suit{Hearts, Diamonds, Clubs, Spades}

func NewGameState(matchID string, players []string, turnTTL time.Duration) GameState {
	return newGameStateWithSource(matchID, players, turnTTL, GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	}, rand.NewSource(time.Now().UnixNano()))
}

// NewGameStateDeterministic creates a game state with seeded RNG for deterministic tests.
func NewGameStateDeterministic(matchID string, players []string, turnTTL time.Duration, seed int64) GameState {
	return newGameStateWithSource(matchID, players, turnTTL, GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	}, rand.NewSource(seed))
}

func NewGameStateWithConfig(matchID string, players []string, turnTTL time.Duration, cfg GameConfig) GameState {
	return newGameStateWithSource(matchID, players, turnTTL, cfg, rand.NewSource(time.Now().UnixNano()))
}

func NewGameStateDeterministicWithConfig(matchID string, players []string, turnTTL time.Duration, cfg GameConfig, seed int64) GameState {
	return newGameStateWithSource(matchID, players, turnTTL, cfg, rand.NewSource(seed))
}

func newGameStateWithSource(matchID string, players []string, turnTTL time.Duration, cfg GameConfig, source rand.Source) GameState {
	deckType := normalizeDeckSize(cfg.DeckSize)
	mode := normalizeMode(cfg.Mode)
	lowerMode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	shulerEnabled := cfg.ShulerEnabled ||
		strings.Contains(lowerMode, "shuler") ||
		strings.Contains(lowerMode, "шулер")
	deck := buildDeck(deckType)
	rand.New(source).Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	attacker := players[0]
	defender := nextPlayerFromOrder(players, attacker)
	trump := deck[len(deck)-1].Suit

	now := time.Now().UTC()
	hands := make(map[string][]Card, len(players))
	for _, playerID := range players {
		hands[playerID] = []Card{}
	}
	dealCardsInOrder(players, hands, &deck, 6)

	shulerPlayerID := ""
	if shulerEnabled {
		if slices.Contains(players, cfg.ShulerPlayerID) {
			shulerPlayerID = cfg.ShulerPlayerID
		} else {
			shulerPlayerID = attacker
		}
	}

	return GameState{
		MatchID:      matchID,
		Status:       StatusPlaying,
		Version:      1,
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
		Mode:         mode,
		DeckType:     deckType,
		ShulerEnabled: shulerEnabled,
		ShulerPlayerID: shulerPlayerID,
		ShulerDetected: false,
		LastActionAt: now,
		Metadata: map[string]interface{}{
			"round_limit": len(hands[defender]),
		},
	}
}

func normalizeMode(mode string) string {
	lower := strings.ToLower(strings.TrimSpace(mode))
	if strings.Contains(lower, "перевод") || strings.Contains(lower, "perevod") {
		return "perevodnoy"
	}
	return "podkidnoy"
}

func normalizeDeckSize(size int) int {
	switch size {
	case 24, 36, 52:
		return size
	default:
		return 36
	}
}

func ranksForDeck(size int) []string {
	switch normalizeDeckSize(size) {
	case 24:
		return rankOrder[7:] // 9..A
	case 36:
		return rankOrder[4:] // 6..A
	default:
		return rankOrder // 2..A
	}
}

func buildDeck(size int) []Card {
	ranks := ranksForDeck(size)
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

func dealCardsInOrderFrom(startPlayerID string, playerOrder []string, hands map[string][]Card, deck *[]Card, target int) {
	if len(playerOrder) == 0 {
		return
	}
	startIndex := 0
	for i, playerID := range playerOrder {
		if playerID == startPlayerID {
			startIndex = i
			break
		}
	}
	ordered := append(slices.Clone(playerOrder[startIndex:]), playerOrder[:startIndex]...)
	dealCardsInOrder(ordered, hands, deck, target)
}

func nextPlayerFromOrder(playerOrder []string, current string) string {
	if len(playerOrder) == 0 {
		return ""
	}
	for i, playerID := range playerOrder {
		if playerID != current {
			continue
		}
		return playerOrder[(i+1)%len(playerOrder)]
	}
	return playerOrder[0]
}
