package ws

import (
	"context"
	"net/http/httptest"
	"testing"

	"durakonline/backend/internal/games/engine"

	"github.com/go-chi/chi/v5"
)

func TestRequestedRoomIDPrefersQueryOverRouteParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/ws/room-42?roomId=query-room", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "path-room")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	if got := requestedRoomID(req); got != "query-room" {
		t.Fatalf("requestedRoomID() = %q, want query-room", got)
	}
}

func TestDecodeClientEventRejectsInvalidJSON(t *testing.T) {
	if _, err := decodeClientEvent([]byte(`{"type":`)); err == nil {
		t.Fatal("expected invalid JSON to return error")
	}
}

func TestNormalizeActionAliases(t *testing.T) {
	cases := map[string]engine.Action{
		"attack_card": engine.ActionAttack,
		"defend_card": engine.ActionDefend,
		"throw_card":  engine.ActionThrow,
		"throw_in":    engine.ActionThrow,
		"take_cards":  engine.ActionTake,
		"pass_turn":   engine.ActionPass,
		"end_round":   engine.ActionPass,
	}

	for input, want := range cases {
		if got := normalizeAction(input); got != want {
			t.Fatalf("normalizeAction(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReadSyncRequestOptionsNormalizesPayload(t *testing.T) {
	options := readSyncRequestOptions(map[string]interface{}{
		"lastKnownVersion":  "-2",
		"lastKnownMatchId":  " match-1 ",
		"supportsStateDiff": "yes",
	})

	if options.lastKnownVersion == nil || *options.lastKnownVersion != 0 {
		t.Fatalf("lastKnownVersion = %v, want 0", options.lastKnownVersion)
	}
	if options.lastKnownMatchID != "match-1" {
		t.Fatalf("lastKnownMatchID = %q, want match-1", options.lastKnownMatchID)
	}
	if !options.supportsStateDiff {
		t.Fatal("supportsStateDiff = false, want true")
	}
}
