package ws

import "durakonline/backend/internal/games/engine"

type ClientEvent struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

type ServerEvent struct {
	Type          string      `json:"type"`
	Payload       interface{} `json:"payload"`
	CorrelationID string      `json:"correlationId,omitempty"`
	Locale        string      `json:"locale,omitempty"`
}

type GameStateDTO struct {
	RoomID           string                   `json:"roomId"`
	MatchID          string                   `json:"matchId"`
	Version          int64                    `json:"version"`
	Phase            string                   `json:"phase"`
	DeckType         int                      `json:"deckType,omitempty"`
	Mode             string                   `json:"mode,omitempty"`
	AttackerPlayerID string                   `json:"attackerPlayerId,omitempty"`
	DefenderPlayerID string                   `json:"defenderPlayerId,omitempty"`
	Players          []PlayerDTO              `json:"players"`
	TableCards       []engine.Card            `json:"tableCards"`
	TrumpSuit        string                   `json:"trumpSuit"`
	TrumpCard        *engine.Card             `json:"trumpCard,omitempty"`
	TurnPlayerID     string                   `json:"turnPlayerId"`
	TurnEndsAt       int64                    `json:"turnEndsAt"`
	Status           string                   `json:"status"`
	Affordances      engine.PlayerAffordances `json:"affordances"`
	WinnerPlayerID   string                   `json:"winnerPlayerId,omitempty"`
	WinnerPlayerIDs  []string                 `json:"winnerPlayerIds,omitempty"`
	IsDraw           bool                     `json:"isDraw,omitempty"`
	FinishGroups     [][]string               `json:"finishGroups,omitempty"`
	Shuler           map[string]any           `json:"shuler,omitempty"`
}

type PlayerDTO struct {
	ID            string        `json:"id"`
	Username      string        `json:"username"` // для совместимости
	DisplayName   string        `json:"displayName"`
	PhotoURL      string        `json:"photoUrl"`
	HandCount     int           `json:"handCount"`
	Hand          []engine.Card `json:"hand,omitempty"`
	IsCurrentTurn bool          `json:"isCurrentTurn"`
}
