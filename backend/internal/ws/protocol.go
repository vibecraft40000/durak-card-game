package ws

import "durakonline/backend/internal/games/engine"

type ClientEvent struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

type ServerEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type GameStateDTO struct {
	RoomID         string        `json:"roomId"`
	MatchID        string        `json:"matchId"`
	Version        int64         `json:"version"`
	Phase          string        `json:"phase"`
	Players        []PlayerDTO   `json:"players"`
	TableCards     []engine.Card `json:"tableCards"`
	TrumpSuit      string        `json:"trumpSuit"`
	TrumpCard      *engine.Card  `json:"trumpCard,omitempty"`
	TurnPlayerID   string        `json:"turnPlayerId"`
	TurnEndsAt     int64         `json:"turnEndsAt"`
	Status         string        `json:"status"`
	WinnerPlayerID string        `json:"winnerPlayerId,omitempty"`
}

type PlayerDTO struct {
	ID          string        `json:"id"`
	Username    string        `json:"username"` // для совместимости
	DisplayName string        `json:"displayName"`
	PhotoURL    string        `json:"photoUrl"`
	HandCount   int           `json:"handCount"`
	Hand        []engine.Card `json:"hand,omitempty"`
	IsCurrentTurn bool        `json:"isCurrentTurn"`
}
