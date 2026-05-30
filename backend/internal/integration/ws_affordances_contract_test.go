package integration

import (
	"encoding/json"
	"testing"
	"time"

	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/ws"
)

func TestWSGameStateIncludesViewerAffordances(t *testing.T) {
	env := newWSContractEnv(t, 30*time.Second)
	players := env.createUsers(t, 2)
	room, _ := env.createInGameRoomWithMatch(t, "podkidnoy", 10, players, engine.GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	})

	conn := env.dialRoom(t, room.ID, players[0].ID)
	defer conn.Close()

	sendClientEvent(t, conn, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]any{
			"roomId": room.ID,
		},
	})

	gameStateEvt := waitForEventType(t, conn, "game_state", 4*time.Second)
	var payload ws.GameStateDTO
	if err := json.Unmarshal(gameStateEvt.Payload, &payload); err != nil {
		t.Fatalf("decode game_state payload: %v", err)
	}

	if !payload.Affordances.CanAct || !payload.Affordances.CanAttack {
		t.Fatalf("expected initial attacker affordances, got %+v", payload.Affordances)
	}
	if payload.Affordances.CanTake || payload.Affordances.CanPass || payload.Affordances.CanThrowIn {
		t.Fatalf("unexpected initial affordances enabled: %+v", payload.Affordances)
	}
	if len(payload.Players) == 0 || len(payload.Players[0].Hand) == 0 {
		t.Fatalf("expected viewer hand in payload, got %+v", payload.Players)
	}
	if len(payload.Affordances.AttackCardIDs) != len(payload.Players[0].Hand) {
		t.Fatalf("expected every initial card to be attackable, got cards=%d affordances=%d", len(payload.Players[0].Hand), len(payload.Affordances.AttackCardIDs))
	}
	if len(payload.Affordances.DefendableTargetIDs) != 0 {
		t.Fatalf("expected no defend targets on empty table, got %v", payload.Affordances.DefendableTargetIDs)
	}
}
