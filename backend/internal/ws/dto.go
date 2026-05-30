package ws

import (
	"context"
	"encoding/json"
	"time"

	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/rooms"
)

func (h *Handler) toGameStateDTO(ctx context.Context, roomID string, state engine.GameState, viewerUserID string) GameStateDTO {
	players := make([]PlayerDTO, 0, len(state.Hands))
	for _, playerID := range state.PlayerOrder {
		hand := state.Hands[playerID]
		var playerHand []engine.Card
		if playerID == viewerUserID {
			playerHand = hand
		}
		shortID := playerID
		if len(shortID) > 4 {
			shortID = shortID[:4]
		}
		displayName := "player-" + shortID
		photoURL := ""
		if rooms.IsBotPlayer(playerID) {
			displayName = "bot"
		} else if user, ok := h.users.GetByID(ctx, playerID); ok {
			if user.DisplayName != "" {
				displayName = user.DisplayName
			}
			photoURL = user.PhotoURL
		}
		players = append(players, PlayerDTO{
			ID:            playerID,
			Username:      displayName,
			DisplayName:   displayName,
			PhotoURL:      photoURL,
			HandCount:     len(hand),
			Hand:          playerHand,
			IsCurrentTurn: state.TurnPlayerID == playerID,
		})
	}
	var trumpCard *engine.Card
	if len(state.Deck) > 0 {
		card := state.Deck[len(state.Deck)-1]
		trumpCard = &card
	}
	phase := string(state.TurnState)
	if state.Status == engine.StatusFinished {
		phase = "result"
	} else if phase == "" {
		phase = "attack"
	}
	return GameStateDTO{
		RoomID:           roomID,
		MatchID:          state.MatchID,
		Version:          state.Version,
		Phase:            phase,
		DeckType:         state.DeckType,
		Mode:             state.Mode,
		AttackerPlayerID: state.AttackerID,
		DefenderPlayerID: state.DefenderID,
		Players:          players,
		TableCards:       state.TableCards,
		TrumpSuit:        string(state.Trump),
		TrumpCard:        trumpCard,
		TurnPlayerID:     state.TurnPlayerID,
		TurnEndsAt:       state.TurnEndsAt.UnixMilli(),
		Status:           string(state.Status),
		Affordances:      engine.ViewAffordances(state, viewerUserID),
		WinnerPlayerID:   state.WinnerPlayer,
		WinnerPlayerIDs:  state.WinnerPlayers,
		IsDraw:           state.IsDraw,
		FinishGroups:     state.FinishGroups,
		Shuler: map[string]any{
			"isWindowOpen": state.ShulerEnabled && !state.ShulerDetected && !state.ShulerWindowUntil.IsZero() && time.Now().UTC().Before(state.ShulerWindowUntil),
			"windowEndsAt": func() int64 {
				if state.ShulerWindowUntil.IsZero() {
					return 0
				}
				return state.ShulerWindowUntil.UnixMilli()
			}(),
			"activePlayers": func() []string {
				if state.ShulerEnabled && !state.ShulerDetected && state.ShulerPlayerID != "" {
					return []string{state.ShulerPlayerID}
				}
				return []string{}
			}(),
		},
	}
}

func (h *Handler) toStateDiffPayload(ctx context.Context, roomID string, state engine.GameState, viewerUserID string, fromVersion int64) map[string]any {
	dto := h.toGameStateDTO(ctx, roomID, state, viewerUserID)

	return map[string]any{
		"roomId":      roomID,
		"matchId":     state.MatchID,
		"fromVersion": fromVersion,
		"toVersion":   dto.Version,
		"patch":       gameStatePatchFromDTO(dto),
	}
}

func gameStatePatchFromDTO(dto GameStateDTO) map[string]any {
	raw, err := json.Marshal(dto)
	if err != nil {
		return map[string]any{
			"version": dto.Version,
			"phase":   dto.Phase,
			"status":  dto.Status,
		}
	}
	var patch map[string]any
	if err := json.Unmarshal(raw, &patch); err != nil {
		return map[string]any{
			"version": dto.Version,
			"phase":   dto.Phase,
			"status":  dto.Status,
		}
	}
	return patch
}
