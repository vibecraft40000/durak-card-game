package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"durakonline/backend/internal/auth"
	"durakonline/backend/internal/games"
	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/ratelimit"
	"durakonline/backend/internal/rooms"
	"durakonline/backend/internal/users"
	"durakonline/backend/internal/ws"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type wsContractEnv struct {
	ctx       context.Context
	authSvc   *auth.Service
	usersRepo *users.Repository
	gamesSvc  *games.Service
	roomsSvc  *rooms.Service
	roomsRepo *rooms.Repository
	redis     *redis.Client
	serverURL string
	jwtSecret string
	nextTG    int64
}

type serverEventEnvelope struct {
	Type          string          `json:"type"`
	Payload       json.RawMessage `json:"payload"`
	CorrelationID string          `json:"correlationId"`
	Locale        string          `json:"locale,omitempty"`
}

func newWSContractEnv(t *testing.T, turnTTL time.Duration) *wsContractEnv {
	t.Helper()

	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	resetData(t, ctx, pg)

	usersRepo := users.NewRepository(pg)
	gamesSvc := games.NewService(pg, redisClient, turnTTL, time.Hour)
	roomsRepo := rooms.NewRepository(redisClient)
	roomsSvc := rooms.NewService(roomsRepo, gamesSvc, nil, 300, true)
	limiter := ratelimit.NewService(redisClient)

	const jwtSecret = "ws-contract-test-secret"
	authSvc := auth.NewService(usersRepo, redisClient, jwtSecret, time.Hour, 24*time.Hour, time.Minute, "")
	hub := ws.NewHub()
	handler := ws.NewHandler(authSvc, roomsSvc, gamesSvc, nil, usersRepo, 300, true, hub, nil, limiter, redisClient, "*", false)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handler.ServeWS)
	server := httptest.NewServer(mux)

	t.Cleanup(func() {
		hub.Drain(200 * time.Millisecond)
		server.Close()
		redisClient.Close()
		pg.Close()
	})

	return &wsContractEnv{
		ctx:       ctx,
		authSvc:   authSvc,
		usersRepo: usersRepo,
		gamesSvc:  gamesSvc,
		roomsSvc:  roomsSvc,
		roomsRepo: roomsRepo,
		redis:     redisClient,
		serverURL: server.URL,
		jwtSecret: jwtSecret,
		nextTG:    700_000,
	}
}

func (e *wsContractEnv) createUsers(t *testing.T, count int) []users.User {
	t.Helper()
	out := make([]users.User, 0, count)
	for i := 0; i < count; i++ {
		e.nextTG++
		username := fmt.Sprintf("ws_u_%d", e.nextTG)
		u, err := e.usersRepo.GetOrCreateByTelegram(e.ctx, e.nextTG, username, "WS", "User", "")
		if err != nil {
			t.Fatalf("create user %d: %v", i, err)
		}
		out = append(out, u)
	}
	return out
}

func (e *wsContractEnv) createInGameRoomWithMatch(
	t *testing.T,
	roomMode string,
	stake float64,
	players []users.User,
	cfg engine.GameConfig,
) (rooms.Room, engine.GameState) {
	t.Helper()
	if len(players) < 2 {
		t.Fatalf("need at least 2 players")
	}

	ownerID := players[0].ID
	room, err := e.roomsSvc.Create(e.ctx, "ws-table", stake, len(players), 36, roomMode, ownerID)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	for i := 1; i < len(players); i++ {
		room, err = e.roomsSvc.Join(e.ctx, room.ID, players[i].ID)
		if err != nil {
			t.Fatalf("join player %d: %v", i, err)
		}
	}

	playerIDs := make([]string, 0, len(players))
	for _, p := range players {
		playerIDs = append(playerIDs, p.ID)
	}

	matchID := uuid.NewString()
	state, err := e.gamesSvc.StartMatchWithConfig(e.ctx, matchID, stake, cfg, playerIDs)
	if err != nil {
		t.Fatalf("start match with config: %v", err)
	}
	room.Status = rooms.StatusInGame
	room.MatchID = matchID
	if err := e.roomsRepo.Save(e.ctx, room); err != nil {
		t.Fatalf("save in-game room: %v", err)
	}
	return room, state
}

func (e *wsContractEnv) dialRoom(t *testing.T, roomID, userID string) *websocket.Conn {
	t.Helper()
	token := mustIssueJWT(t, e.jwtSecret, userID)
	wsURL := strings.Replace(e.serverURL, "http://", "ws://", 1) + "/ws"
	return dialRoomURLWithToken(t, wsURL, token, roomID)
}

func (e *wsContractEnv) issueWSTicket(t *testing.T, userID, roomID string) string {
	t.Helper()
	ticket, err := e.authSvc.IssueWSTicket(e.ctx, userID, roomID)
	if err != nil {
		t.Fatalf("issue ws ticket: %v", err)
	}
	return ticket
}

func dialRoomURLWithToken(t *testing.T, wsURL, token, roomID string) *websocket.Conn {
	t.Helper()
	query := url.Values{}
	query.Set("token", token)
	query.Set("roomId", roomID)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL+"?"+query.Encode(), nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("dial ws failed: %v (status=%s)", err, resp.Status)
		}
		t.Fatalf("dial ws failed: %v", err)
	}
	return conn
}

func dialRoomURLWithTokenAndLocale(t *testing.T, wsURL, token, roomID, locale string) *websocket.Conn {
	t.Helper()
	query := url.Values{}
	query.Set("token", token)
	query.Set("roomId", roomID)
	if locale != "" {
		query.Set("locale", locale)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL+"?"+query.Encode(), nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("dial ws failed: %v (status=%s)", err, resp.Status)
		}
		t.Fatalf("dial ws failed: %v", err)
	}
	return conn
}

func dialRoomURLWithTicket(t *testing.T, wsURL, ticket, roomID string) *websocket.Conn {
	t.Helper()
	query := url.Values{}
	query.Set("ticket", ticket)
	query.Set("roomId", roomID)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL+"?"+query.Encode(), nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("dial ws failed: %v (status=%s)", err, resp.Status)
		}
		t.Fatalf("dial ws failed: %v", err)
	}
	return conn
}

func mustIssueJWT(t *testing.T, secret, userID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signed
}

func sendClientEvent(t *testing.T, conn *websocket.Conn, event ws.ClientEvent) {
	t.Helper()
	if err := conn.WriteJSON(event); err != nil {
		t.Fatalf("write ws event %s: %v", event.Type, err)
	}
}

func readNextServerEvent(t *testing.T, conn *websocket.Conn, deadline time.Time) serverEventEnvelope {
	t.Helper()
	remaining := time.Until(deadline)
	if remaining <= 0 {
		t.Fatal("read deadline exceeded")
	}
	if err := conn.SetReadDeadline(time.Now().Add(remaining)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ws message: %v", err)
	}
	var evt serverEventEnvelope
	if err := json.Unmarshal(raw, &evt); err != nil {
		t.Fatalf("decode ws event: %v", err)
	}
	return evt
}

func waitForEventType(t *testing.T, conn *websocket.Conn, eventType string, timeout time.Duration) serverEventEnvelope {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		evt := readNextServerEvent(t, conn, deadline)
		if evt.Type == eventType {
			return evt
		}
	}
}

func waitForTimeoutResult(t *testing.T, svc *games.Service, ctx context.Context, matchID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		results := svc.HandleTimeouts(ctx)
		for _, result := range results {
			if result.MatchID == matchID {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timeout was not applied for match %s", matchID)
}

func tryReadServerEvent(conn *websocket.Conn, timeout time.Duration) (serverEventEnvelope, bool, error) {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			return serverEventEnvelope{}, false, nil
		}
		if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
			return serverEventEnvelope{}, false, nil
		}
		return serverEventEnvelope{}, false, err
	}
	var evt serverEventEnvelope
	if err := json.Unmarshal(raw, &evt); err != nil {
		return serverEventEnvelope{}, false, err
	}
	return evt, true, nil
}

func parseMoveVersionFromEventID(eventID string) int64 {
	idx := strings.LastIndex(eventID, ":v")
	if idx < 0 || idx+2 >= len(eventID) {
		return 0
	}
	version, err := strconv.ParseInt(eventID[idx+2:], 10, 64)
	if err != nil || version < 0 {
		return 0
	}
	return version
}

func pickAttackDefensePair(state engine.GameState, attackerID, defenderID string) (string, string, bool) {
	attackerHand := state.Hands[attackerID]
	defenderHand := state.Hands[defenderID]
	for _, attackCard := range attackerHand {
		defendCard, ok := pickDefenseCard(defenderHand, attackCard, state.Trump)
		if !ok {
			continue
		}
		return attackCard.ID, defendCard.ID, true
	}
	return "", "", false
}

func TestWSEnvelopeIncludesCorrelationIDAndLocale(t *testing.T) {
	env := newWSContractEnv(t, 30*time.Second)
	players := env.createUsers(t, 2)
	room, _ := env.createInGameRoomWithMatch(t, "podkidnoy", 10, players, engine.GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	})

	token := mustIssueJWT(t, env.jwtSecret, players[0].ID)
	wsURL := strings.Replace(env.serverURL, "http://", "ws://", 1) + "/ws"
	conn := dialRoomURLWithTokenAndLocale(t, wsURL, token, room.ID, "uk")
	defer conn.Close()

	sendClientEvent(t, conn, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})

	gameStateEvt := waitForEventType(t, conn, "game_state", 4*time.Second)
	if gameStateEvt.CorrelationID == "" {
		t.Fatal("expected non-empty correlationId in ws envelope")
	}
	if gameStateEvt.Locale != "uk" {
		t.Fatalf("expected locale=uk in ws envelope, got %q", gameStateEvt.Locale)
	}
}

func TestWSTicketAuthIsSingleUseAndRoomBound(t *testing.T) {
	env := newWSContractEnv(t, 30*time.Second)
	players := env.createUsers(t, 2)
	room, _ := env.createInGameRoomWithMatch(t, "podkidnoy", 10, players, engine.GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	})

	wsURL := strings.Replace(env.serverURL, "http://", "ws://", 1) + "/ws"
	ticket := env.issueWSTicket(t, players[0].ID, room.ID)
	conn := dialRoomURLWithTicket(t, wsURL, ticket, room.ID)
	defer conn.Close()

	sendClientEvent(t, conn, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	_ = waitForEventType(t, conn, "game_state", 4*time.Second)

	reusedQuery := url.Values{}
	reusedQuery.Set("ticket", ticket)
	reusedQuery.Set("roomId", room.ID)
	if _, resp, err := websocket.DefaultDialer.Dial(wsURL+"?"+reusedQuery.Encode(), nil); err == nil {
		t.Fatal("expected second dial with same ticket to fail")
	} else if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		if resp == nil {
			t.Fatalf("expected 401 on reused ticket, got nil response (err=%v)", err)
		}
		t.Fatalf("expected 401 on reused ticket, got %d (err=%v)", resp.StatusCode, err)
	}

	wrongRoomTicket := env.issueWSTicket(t, players[0].ID, room.ID)
	wrongRoomQuery := url.Values{}
	wrongRoomQuery.Set("ticket", wrongRoomTicket)
	wrongRoomQuery.Set("roomId", "room-other")
	if _, resp, err := websocket.DefaultDialer.Dial(wsURL+"?"+wrongRoomQuery.Encode(), nil); err == nil {
		t.Fatal("expected dial with wrong room ticket binding to fail")
	} else if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		if resp == nil {
			t.Fatalf("expected 401 on wrong-room ticket, got nil response (err=%v)", err)
		}
		t.Fatalf("expected 401 on wrong-room ticket, got %d (err=%v)", resp.StatusCode, err)
	}
}

func TestWSLateShulerReportReturnsInvalidAction(t *testing.T) {
	env := newWSContractEnv(t, 30*time.Second)
	players := env.createUsers(t, 2)
	room, state := env.createInGameRoomWithMatch(t, "podkidnoy_shuler", 10, players, engine.GameConfig{
		DeckSize:       36,
		Mode:           "podkidnoy",
		ShulerEnabled:  true,
		ShulerPlayerID: players[1].ID,
	})

	attackCardID := state.Hands[players[0].ID][0].ID
	state, _, err := env.gamesSvc.Apply(env.ctx, room.MatchID, players[0].ID, engine.ActionAttack, attackCardID, nil, "")
	if err != nil {
		t.Fatalf("attack before shuler play: %v", err)
	}
	shulerCardID := state.Hands[players[1].ID][0].ID
	state, _, err = env.gamesSvc.Apply(env.ctx, room.MatchID, players[1].ID, engine.ActionShulerPlay, shulerCardID, nil, "")
	if err != nil {
		t.Fatalf("shuler play: %v", err)
	}
	if state.ShulerWindowUntil.IsZero() {
		t.Fatal("expected opened shuler window")
	}

	waitUntil := state.ShulerWindowUntil.Add(120 * time.Millisecond)
	for time.Now().UTC().Before(waitUntil) {
		time.Sleep(20 * time.Millisecond)
	}

	conn := env.dialRoom(t, room.ID, players[0].ID)
	defer conn.Close()

	actionID := "late-report-action"
	sendClientEvent(t, conn, ws.ClientEvent{
		Type: "make_move",
		Payload: map[string]interface{}{
			"roomId":          room.ID,
			"action":          "shuler_report",
			"expectedVersion": float64(state.Version),
			"actionId":        actionID,
		},
	})

	errEvt := waitForEventType(t, conn, "error", 4*time.Second)
	var payload struct {
		Message   string `json:"message"`
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(errEvt.Payload, &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.ErrorCode != "INVALID_ACTION" {
		t.Fatalf("expected INVALID_ACTION, got %q (message=%q)", payload.ErrorCode, payload.Message)
	}
	if !strings.Contains(strings.ToLower(payload.Message), "cannot report shuler") {
		t.Fatalf("unexpected error message: %q", payload.Message)
	}
}

func TestWSVersionMismatchEventContractAfterTimeout(t *testing.T) {
	env := newWSContractEnv(t, 300*time.Millisecond)
	players := env.createUsers(t, 3)
	room, state := env.createInGameRoomWithMatch(t, "podkidnoy", 10, players, engine.GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	})

	attackCardID, defendCardID, throwCardID, ok := pickThrowWindowScenario(state)
	if !ok {
		t.Fatal("unable to prepare throw-window scenario for websocket test")
	}

	state, _, err := env.gamesSvc.Apply(env.ctx, room.MatchID, players[0].ID, engine.ActionAttack, attackCardID, nil, "")
	if err != nil {
		t.Fatalf("attack before throw window: %v", err)
	}
	state, _, err = env.gamesSvc.Apply(env.ctx, room.MatchID, players[1].ID, engine.ActionDefend, defendCardID, nil, "")
	if err != nil {
		t.Fatalf("defend before throw window: %v", err)
	}
	staleVersion := state.Version

	conn := env.dialRoom(t, room.ID, players[2].ID)
	defer conn.Close()

	waitForTimeoutResult(t, env.gamesSvc, env.ctx, room.MatchID, 4*time.Second)

	const actionID = "stale-throw-retry"
	sendClientEvent(t, conn, ws.ClientEvent{
		Type: "make_move",
		Payload: map[string]interface{}{
			"roomId":          room.ID,
			"action":          "throw_card",
			"cardId":          throwCardID,
			"expectedVersion": float64(staleVersion),
			"actionId":        actionID,
		},
	})

	deadline := time.Now().Add(4 * time.Second)
	relevant := make([]serverEventEnvelope, 0, 2)
	seen := map[string]bool{}
	for len(seen) < 2 {
		evt := readNextServerEvent(t, conn, deadline)
		if evt.Type == "game_state" || evt.Type == "version_mismatch" {
			relevant = append(relevant, evt)
			seen[evt.Type] = true
		}
	}
	if len(relevant) < 2 || relevant[0].Type != "game_state" || relevant[1].Type != "version_mismatch" {
		got := make([]string, 0, len(relevant))
		for _, evt := range relevant {
			got = append(got, evt.Type)
		}
		t.Fatalf("expected [game_state, version_mismatch], got %v", got)
	}

	var payload struct {
		RoomID   string `json:"roomId"`
		Action   string `json:"action"`
		CardID   string `json:"cardId"`
		ActionID string `json:"actionId"`
	}
	if err := json.Unmarshal(relevant[1].Payload, &payload); err != nil {
		t.Fatalf("decode version_mismatch payload: %v", err)
	}
	if payload.RoomID != room.ID {
		t.Fatalf("roomId mismatch: got %s want %s", payload.RoomID, room.ID)
	}
	if payload.Action != "throw_card" {
		t.Fatalf("action mismatch: got %s want throw_card", payload.Action)
	}
	if payload.CardID != throwCardID {
		t.Fatalf("cardId mismatch: got %s want %s", payload.CardID, throwCardID)
	}
	if payload.ActionID != actionID {
		t.Fatalf("actionId mismatch: got %s want %s", payload.ActionID, actionID)
	}
}

func TestWSReconnectReplayAndNoDuplicateOnFreshSync(t *testing.T) {
	env := newWSContractEnv(t, 30*time.Second)
	players := env.createUsers(t, 2)
	room, state := env.createInGameRoomWithMatch(t, "podkidnoy", 10, players, engine.GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	})

	attackCardID, defendCardID, ok := pickAttackDefensePair(state, players[0].ID, players[1].ID)
	if !ok {
		t.Fatal("unable to find attack/defend pair for replay test")
	}

	conn1 := env.dialRoom(t, room.ID, players[0].ID)
	defer conn1.Close()
	conn2 := env.dialRoom(t, room.ID, players[1].ID)
	defer conn2.Close()

	sendClientEvent(t, conn1, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	sendClientEvent(t, conn2, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	_ = waitForEventType(t, conn1, "game_state", 4*time.Second)
	_ = waitForEventType(t, conn2, "game_state", 4*time.Second)

	sendClientEvent(t, conn1, ws.ClientEvent{
		Type: "make_move",
		Payload: map[string]interface{}{
			"roomId": room.ID,
			"action": "attack_card",
			"cardId": attackCardID,
		},
	})
	sendClientEvent(t, conn2, ws.ClientEvent{
		Type: "make_move",
		Payload: map[string]interface{}{
			"roomId": room.ID,
			"action": "defend_card",
			"cardId": defendCardID,
		},
	})

	deadline := time.Now().Add(4 * time.Second)
	for {
		current, err := env.gamesSvc.GetState(env.ctx, room.MatchID)
		if err == nil && current.Version >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("match state did not reach expected version 3")
		}
		time.Sleep(20 * time.Millisecond)
	}

	connReplay := env.dialRoom(t, room.ID, players[0].ID)
	defer connReplay.Close()
	sendClientEvent(t, connReplay, ws.ClientEvent{
		Type: "reconnect",
		Payload: map[string]interface{}{
			"roomId":           room.ID,
			"lastKnownVersion": float64(1),
		},
	})

	type stateSyncPayload struct {
		RoomID      string `json:"roomId"`
		MatchID     string `json:"matchId"`
		FromVersion int64  `json:"fromVersion"`
		ToVersion   int64  `json:"toVersion"`
		Mode        string `json:"mode"`
		ReplayCount int    `json:"replayCount"`
	}
	var syncPayload stateSyncPayload
	gotSync := false
	replayedVersions := make([]int64, 0, 4)
	finalStateVersion := int64(0)

	readDeadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(readDeadline) {
		evt := readNextServerEvent(t, connReplay, readDeadline)
		switch evt.Type {
		case "state_sync":
			var payload stateSyncPayload
			if err := json.Unmarshal(evt.Payload, &payload); err != nil {
				t.Fatalf("decode state_sync payload: %v", err)
			}
			if payload.Mode == "replay" {
				syncPayload = payload
				gotSync = true
			}
		case "move_applied":
			if !gotSync {
				continue
			}
			var payload struct {
				EventID string `json:"eventId"`
			}
			if err := json.Unmarshal(evt.Payload, &payload); err != nil {
				t.Fatalf("decode move_applied payload: %v", err)
			}
			if version := parseMoveVersionFromEventID(payload.EventID); version > 0 {
				replayedVersions = append(replayedVersions, version)
			}
		case "game_state":
			if !gotSync {
				continue
			}
			var payload struct {
				Version int64 `json:"version"`
			}
			if err := json.Unmarshal(evt.Payload, &payload); err != nil {
				t.Fatalf("decode game_state payload: %v", err)
			}
			finalStateVersion = payload.Version
		}
		if gotSync && len(replayedVersions) >= syncPayload.ReplayCount && finalStateVersion == syncPayload.ToVersion {
			break
		}
	}

	if !gotSync {
		t.Fatal("expected state_sync in replay mode on reconnect")
	}
	if syncPayload.FromVersion != 1 {
		t.Fatalf("unexpected fromVersion: got %d want 1", syncPayload.FromVersion)
	}
	if syncPayload.ToVersion < 3 {
		t.Fatalf("unexpected toVersion: got %d want >=3", syncPayload.ToVersion)
	}
	if syncPayload.ReplayCount <= 0 {
		t.Fatalf("expected replayCount > 0, got %d", syncPayload.ReplayCount)
	}
	if len(replayedVersions) < syncPayload.ReplayCount {
		t.Fatalf("expected at least %d replayed moves, got %d", syncPayload.ReplayCount, len(replayedVersions))
	}
	for i := 1; i < syncPayload.ReplayCount; i++ {
		if replayedVersions[i] != replayedVersions[i-1]+1 {
			t.Fatalf("replay versions are not contiguous: %v", replayedVersions[:syncPayload.ReplayCount])
		}
	}
	if replayedVersions[0] != syncPayload.FromVersion+1 {
		t.Fatalf("replay should start from fromVersion+1; got %d", replayedVersions[0])
	}
	if finalStateVersion != syncPayload.ToVersion {
		t.Fatalf("final game_state version mismatch: got %d want %d", finalStateVersion, syncPayload.ToVersion)
	}

	sendClientEvent(t, connReplay, ws.ClientEvent{
		Type: "sync_request",
		Payload: map[string]interface{}{
			"roomId":           room.ID,
			"lastKnownVersion": float64(syncPayload.ToVersion),
		},
	})

	gotNoopSync := false
	gotNoopState := false
	checkDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(checkDeadline) {
		evt := readNextServerEvent(t, connReplay, checkDeadline)
		switch evt.Type {
		case "state_sync":
			var payload stateSyncPayload
			if err := json.Unmarshal(evt.Payload, &payload); err != nil {
				t.Fatalf("decode state_sync payload: %v", err)
			}
			if payload.Mode == "noop" {
				gotNoopSync = true
			}
		case "game_state":
			gotNoopState = true
		case "move_applied":
			t.Fatalf("unexpected move_applied on fresh sync_request (duplicate replay): %s", string(evt.Payload))
		}
		if gotNoopSync && gotNoopState {
			break
		}
	}
	if !gotNoopSync {
		t.Fatal("expected noop state_sync on fresh sync_request")
	}
	if !gotNoopState {
		t.Fatal("expected game_state on fresh sync_request")
	}

	if evt, ok, err := tryReadServerEvent(connReplay, 250*time.Millisecond); err != nil {
		t.Fatalf("read after noop sync: %v", err)
	} else if ok && evt.Type == "move_applied" {
		t.Fatalf("unexpected trailing duplicate move_applied after noop sync: %s", string(evt.Payload))
	}
}

func TestWSReconnectReplaySurvivesHandlerRestart(t *testing.T) {
	env := newWSContractEnv(t, 30*time.Second)
	players := env.createUsers(t, 2)
	room, state := env.createInGameRoomWithMatch(t, "podkidnoy", 10, players, engine.GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	})
	attackCardID, defendCardID, ok := pickAttackDefensePair(state, players[0].ID, players[1].ID)
	if !ok {
		t.Fatal("unable to find attack/defend pair for restart replay test")
	}

	conn1 := env.dialRoom(t, room.ID, players[0].ID)
	defer conn1.Close()
	conn2 := env.dialRoom(t, room.ID, players[1].ID)
	defer conn2.Close()

	sendClientEvent(t, conn1, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	sendClientEvent(t, conn2, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	_ = waitForEventType(t, conn1, "game_state", 4*time.Second)
	_ = waitForEventType(t, conn2, "game_state", 4*time.Second)

	sendClientEvent(t, conn1, ws.ClientEvent{
		Type: "make_move",
		Payload: map[string]interface{}{
			"roomId": room.ID,
			"action": "attack_card",
			"cardId": attackCardID,
		},
	})
	sendClientEvent(t, conn2, ws.ClientEvent{
		Type: "make_move",
		Payload: map[string]interface{}{
			"roomId": room.ID,
			"action": "defend_card",
			"cardId": defendCardID,
		},
	})

	waitDeadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(waitDeadline) {
		latest, err := env.gamesSvc.GetState(env.ctx, room.MatchID)
		if err == nil && latest.Version >= 3 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	hubRestarted := ws.NewHub()
	defer hubRestarted.Drain(200 * time.Millisecond)
	authRestarted := auth.NewService(env.usersRepo, env.redis, env.jwtSecret, time.Hour, 24*time.Hour, time.Minute, "")
	limiterRestarted := ratelimit.NewService(env.redis)
	handlerRestarted := ws.NewHandler(authRestarted, env.roomsSvc, env.gamesSvc, nil, env.usersRepo, 300, true, hubRestarted, nil, limiterRestarted, env.redis, "*", false)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handlerRestarted.ServeWS)
	serverRestarted := httptest.NewServer(mux)
	defer serverRestarted.Close()

	token := mustIssueJWT(t, env.jwtSecret, players[0].ID)
	wsURL := strings.Replace(serverRestarted.URL, "http://", "ws://", 1) + "/ws"
	connRestarted := dialRoomURLWithToken(t, wsURL, token, room.ID)
	defer connRestarted.Close()

	sendClientEvent(t, connRestarted, ws.ClientEvent{
		Type: "reconnect",
		Payload: map[string]interface{}{
			"roomId":           room.ID,
			"lastKnownVersion": float64(1),
		},
	})

	type stateSyncPayload struct {
		Mode        string `json:"mode"`
		FromVersion int64  `json:"fromVersion"`
		ToVersion   int64  `json:"toVersion"`
		ReplayCount int    `json:"replayCount"`
	}
	var syncPayload stateSyncPayload
	gotReplaySync := false
	replayedVersions := make([]int64, 0, 4)
	finalStateVersion := int64(0)

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		evt := readNextServerEvent(t, connRestarted, deadline)
		switch evt.Type {
		case "state_sync":
			var payload stateSyncPayload
			if err := json.Unmarshal(evt.Payload, &payload); err != nil {
				t.Fatalf("decode state_sync payload: %v", err)
			}
			if payload.Mode == "replay" {
				gotReplaySync = true
				syncPayload = payload
			}
		case "move_applied":
			if !gotReplaySync {
				continue
			}
			var payload struct {
				EventID string `json:"eventId"`
			}
			if err := json.Unmarshal(evt.Payload, &payload); err != nil {
				t.Fatalf("decode move_applied payload: %v", err)
			}
			if version := parseMoveVersionFromEventID(payload.EventID); version > 0 {
				replayedVersions = append(replayedVersions, version)
			}
		case "game_state":
			if !gotReplaySync {
				continue
			}
			var payload struct {
				Version int64 `json:"version"`
			}
			if err := json.Unmarshal(evt.Payload, &payload); err != nil {
				t.Fatalf("decode game_state payload: %v", err)
			}
			finalStateVersion = payload.Version
		}
		if gotReplaySync && len(replayedVersions) >= syncPayload.ReplayCount && finalStateVersion == syncPayload.ToVersion {
			break
		}
	}

	if !gotReplaySync {
		t.Fatal("expected replay state_sync after handler restart")
	}
	if syncPayload.FromVersion != 1 {
		t.Fatalf("unexpected fromVersion after restart: got %d want 1", syncPayload.FromVersion)
	}
	if syncPayload.ReplayCount < 2 {
		t.Fatalf("expected at least 2 replayed moves after restart, got %d", syncPayload.ReplayCount)
	}
	if len(replayedVersions) < syncPayload.ReplayCount {
		t.Fatalf("replayed move count mismatch: got %d want >= %d", len(replayedVersions), syncPayload.ReplayCount)
	}
	if replayedVersions[0] != syncPayload.FromVersion+1 {
		t.Fatalf("replay should start from fromVersion+1; got %d", replayedVersions[0])
	}
	for i := 1; i < syncPayload.ReplayCount; i++ {
		if replayedVersions[i] != replayedVersions[i-1]+1 {
			t.Fatalf("replay versions must be contiguous, got %v", replayedVersions[:syncPayload.ReplayCount])
		}
	}
	if finalStateVersion != syncPayload.ToVersion {
		t.Fatalf("final game_state version mismatch after restart: got %d want %d", finalStateVersion, syncPayload.ToVersion)
	}
}

func TestWSReconnectSnapshotIncludesTailReplayWhenFullReplayUnavailable(t *testing.T) {
	env := newWSContractEnv(t, 30*time.Second)
	players := env.createUsers(t, 2)
	room, state := env.createInGameRoomWithMatch(t, "podkidnoy", 10, players, engine.GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	})
	attackCardID, defendCardID, ok := pickAttackDefensePair(state, players[0].ID, players[1].ID)
	if !ok {
		t.Fatal("unable to find attack/defend pair for tail replay test")
	}

	conn1 := env.dialRoom(t, room.ID, players[0].ID)
	defer conn1.Close()
	conn2 := env.dialRoom(t, room.ID, players[1].ID)
	defer conn2.Close()

	sendClientEvent(t, conn1, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	sendClientEvent(t, conn2, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	_ = waitForEventType(t, conn1, "game_state", 4*time.Second)
	_ = waitForEventType(t, conn2, "game_state", 4*time.Second)

	sendClientEvent(t, conn1, ws.ClientEvent{
		Type: "make_move",
		Payload: map[string]interface{}{
			"roomId": room.ID,
			"action": "attack_card",
			"cardId": attackCardID,
		},
	})
	sendClientEvent(t, conn2, ws.ClientEvent{
		Type: "make_move",
		Payload: map[string]interface{}{
			"roomId": room.ID,
			"action": "defend_card",
			"cardId": defendCardID,
		},
	})

	waitDeadline := time.Now().Add(4 * time.Second)
	var latestState engine.GameState
	for time.Now().Before(waitDeadline) {
		current, err := env.gamesSvc.GetState(env.ctx, room.MatchID)
		if err == nil && current.Version >= 3 {
			latestState = current
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if latestState.Version < 3 {
		t.Fatalf("match state did not reach version >=3, got %d", latestState.Version)
	}

	replayKey := "ws:replay:" + room.MatchID
	rawEntries, err := env.redis.LRange(env.ctx, replayKey, 0, -1).Result()
	if err != nil {
		t.Fatalf("read replay entries: %v", err)
	}
	if len(rawEntries) < 2 {
		t.Fatalf("expected at least 2 replay entries, got %d", len(rawEntries))
	}
	var latestReplayRaw string
	var latestReplayVersion int64
	for _, raw := range rawEntries {
		var entry struct {
			Version int64 `json:"version"`
		}
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			continue
		}
		if entry.Version > latestReplayVersion {
			latestReplayVersion = entry.Version
			latestReplayRaw = raw
		}
	}
	if latestReplayRaw == "" || latestReplayVersion < latestState.Version {
		t.Fatalf("failed to find latest replay entry >= state version, latestReplay=%d state=%d", latestReplayVersion, latestState.Version)
	}
	if err := env.redis.Del(env.ctx, replayKey).Err(); err != nil {
		t.Fatalf("clear replay key: %v", err)
	}
	if err := env.redis.RPush(env.ctx, replayKey, latestReplayRaw).Err(); err != nil {
		t.Fatalf("write trimmed replay key: %v", err)
	}
	if err := env.redis.Expire(env.ctx, replayKey, 6*time.Hour).Err(); err != nil {
		t.Fatalf("expire trimmed replay key: %v", err)
	}

	hubRestarted := ws.NewHub()
	defer hubRestarted.Drain(200 * time.Millisecond)
	authRestarted := auth.NewService(env.usersRepo, env.redis, env.jwtSecret, time.Hour, 24*time.Hour, time.Minute, "")
	limiterRestarted := ratelimit.NewService(env.redis)
	handlerRestarted := ws.NewHandler(authRestarted, env.roomsSvc, env.gamesSvc, nil, env.usersRepo, 300, true, hubRestarted, nil, limiterRestarted, env.redis, "*", false)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handlerRestarted.ServeWS)
	serverRestarted := httptest.NewServer(mux)
	defer serverRestarted.Close()

	token := mustIssueJWT(t, env.jwtSecret, players[0].ID)
	wsURL := strings.Replace(serverRestarted.URL, "http://", "ws://", 1) + "/ws"
	connRestarted := dialRoomURLWithToken(t, wsURL, token, room.ID)
	defer connRestarted.Close()

	sendClientEvent(t, connRestarted, ws.ClientEvent{
		Type: "reconnect",
		Payload: map[string]interface{}{
			"roomId":           room.ID,
			"lastKnownVersion": float64(1),
		},
	})

	type stateSyncPayload struct {
		Mode              string `json:"mode"`
		FromVersion       int64  `json:"fromVersion"`
		ToVersion         int64  `json:"toVersion"`
		ReplayCount       int    `json:"replayCount"`
		ReplayFromVersion int64  `json:"replayFromVersion"`
	}

	var syncPayload stateSyncPayload
	gotSync := false
	replayedVersions := make([]int64, 0, 4)
	finalStateVersion := int64(0)

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		evt := readNextServerEvent(t, connRestarted, deadline)
		switch evt.Type {
		case "state_sync":
			if err := json.Unmarshal(evt.Payload, &syncPayload); err != nil {
				t.Fatalf("decode state_sync payload: %v", err)
			}
			gotSync = true
		case "move_applied":
			if !gotSync {
				continue
			}
			var payload struct {
				EventID string `json:"eventId"`
			}
			if err := json.Unmarshal(evt.Payload, &payload); err != nil {
				t.Fatalf("decode move_applied payload: %v", err)
			}
			if version := parseMoveVersionFromEventID(payload.EventID); version > 0 {
				replayedVersions = append(replayedVersions, version)
			}
		case "game_state":
			if !gotSync {
				continue
			}
			var payload struct {
				Version int64 `json:"version"`
			}
			if err := json.Unmarshal(evt.Payload, &payload); err != nil {
				t.Fatalf("decode game_state payload: %v", err)
			}
			finalStateVersion = payload.Version
		}
		if gotSync && finalStateVersion == syncPayload.ToVersion && len(replayedVersions) >= syncPayload.ReplayCount {
			break
		}
	}

	if !gotSync {
		t.Fatal("expected state_sync on reconnect")
	}
	if syncPayload.Mode != "snapshot" {
		t.Fatalf("expected snapshot mode for partial replay fallback, got %s", syncPayload.Mode)
	}
	if syncPayload.FromVersion != 1 {
		t.Fatalf("unexpected fromVersion: got %d want 1", syncPayload.FromVersion)
	}
	if syncPayload.ReplayCount <= 0 {
		t.Fatalf("expected replayCount > 0 in snapshot fallback, got %d", syncPayload.ReplayCount)
	}
	if len(replayedVersions) < syncPayload.ReplayCount {
		t.Fatalf("replayed move count mismatch: got %d want >= %d", len(replayedVersions), syncPayload.ReplayCount)
	}
	if syncPayload.ReplayFromVersion <= syncPayload.FromVersion+1 {
		t.Fatalf("expected tail replay to start after fromVersion+1, got replayFromVersion=%d fromVersion=%d", syncPayload.ReplayFromVersion, syncPayload.FromVersion)
	}
	if replayedVersions[0] != syncPayload.ReplayFromVersion {
		t.Fatalf("expected first replayed version %d, got %d", syncPayload.ReplayFromVersion, replayedVersions[0])
	}
	if replayedVersions[syncPayload.ReplayCount-1] != syncPayload.ToVersion {
		t.Fatalf("expected last replayed version %d, got %d", syncPayload.ToVersion, replayedVersions[syncPayload.ReplayCount-1])
	}
	if finalStateVersion != syncPayload.ToVersion {
		t.Fatalf("final game_state version mismatch: got %d want %d", finalStateVersion, syncPayload.ToVersion)
	}
}

func TestWSReconnectSnapshotWithoutReplayEmitsStateDiff(t *testing.T) {
	env := newWSContractEnv(t, 30*time.Second)
	players := env.createUsers(t, 2)
	room, state := env.createInGameRoomWithMatch(t, "podkidnoy", 10, players, engine.GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	})
	attackCardID, _, ok := pickAttackDefensePair(state, players[0].ID, players[1].ID)
	if !ok {
		t.Fatal("unable to find attack card for state_diff snapshot test")
	}

	conn1 := env.dialRoom(t, room.ID, players[0].ID)
	defer conn1.Close()
	conn2 := env.dialRoom(t, room.ID, players[1].ID)
	defer conn2.Close()

	sendClientEvent(t, conn1, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	sendClientEvent(t, conn2, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	_ = waitForEventType(t, conn1, "game_state", 4*time.Second)
	_ = waitForEventType(t, conn2, "game_state", 4*time.Second)

	sendClientEvent(t, conn1, ws.ClientEvent{
		Type: "make_move",
		Payload: map[string]interface{}{
			"roomId": room.ID,
			"action": "attack_card",
			"cardId": attackCardID,
		},
	})

	waitDeadline := time.Now().Add(4 * time.Second)
	var latestVersion int64
	for time.Now().Before(waitDeadline) {
		current, err := env.gamesSvc.GetState(env.ctx, room.MatchID)
		if err == nil && current.Version >= 2 {
			latestVersion = current.Version
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if latestVersion < 2 {
		t.Fatalf("expected match version >=2, got %d", latestVersion)
	}

	replayKey := "ws:replay:" + room.MatchID
	if err := env.redis.Del(env.ctx, replayKey).Err(); err != nil {
		t.Fatalf("clear replay redis key: %v", err)
	}

	hubRestarted := ws.NewHub()
	defer hubRestarted.Drain(200 * time.Millisecond)
	authRestarted := auth.NewService(env.usersRepo, env.redis, env.jwtSecret, time.Hour, 24*time.Hour, time.Minute, "")
	limiterRestarted := ratelimit.NewService(env.redis)
	handlerRestarted := ws.NewHandler(authRestarted, env.roomsSvc, env.gamesSvc, nil, env.usersRepo, 300, true, hubRestarted, nil, limiterRestarted, env.redis, "*", false)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handlerRestarted.ServeWS)
	serverRestarted := httptest.NewServer(mux)
	defer serverRestarted.Close()

	token := mustIssueJWT(t, env.jwtSecret, players[0].ID)
	wsURL := strings.Replace(serverRestarted.URL, "http://", "ws://", 1) + "/ws"
	connRestarted := dialRoomURLWithToken(t, wsURL, token, room.ID)
	defer connRestarted.Close()

	sendClientEvent(t, connRestarted, ws.ClientEvent{
		Type: "reconnect",
		Payload: map[string]interface{}{
			"roomId":           room.ID,
			"lastKnownVersion": float64(1),
		},
	})

	type stateSyncPayload struct {
		Mode        string `json:"mode"`
		FromVersion int64  `json:"fromVersion"`
		ToVersion   int64  `json:"toVersion"`
		ReplayCount int    `json:"replayCount"`
	}
	type stateDiffPayload struct {
		RoomID      string          `json:"roomId"`
		MatchID     string          `json:"matchId"`
		FromVersion int64           `json:"fromVersion"`
		ToVersion   int64           `json:"toVersion"`
		Patch       json.RawMessage `json:"patch"`
	}

	var (
		gotSync      bool
		gotStateDiff bool
		gotGameState bool
		syncPayload  stateSyncPayload
		diffPayload  stateDiffPayload
	)

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		evt := readNextServerEvent(t, connRestarted, deadline)
		switch evt.Type {
		case "state_sync":
			if err := json.Unmarshal(evt.Payload, &syncPayload); err != nil {
				t.Fatalf("decode state_sync payload: %v", err)
			}
			gotSync = true
		case "state_diff":
			if err := json.Unmarshal(evt.Payload, &diffPayload); err != nil {
				t.Fatalf("decode state_diff payload: %v", err)
			}
			gotStateDiff = true
		case "game_state":
			gotGameState = true
		}
		if gotSync && gotStateDiff && gotGameState {
			break
		}
	}

	if !gotSync {
		t.Fatal("expected state_sync")
	}
	if syncPayload.Mode != "snapshot" {
		t.Fatalf("expected snapshot mode when replay unavailable, got %s", syncPayload.Mode)
	}
	if syncPayload.ReplayCount != 0 {
		t.Fatalf("expected replayCount=0 in snapshot without replay, got %d", syncPayload.ReplayCount)
	}
	if !gotStateDiff {
		t.Fatal("expected state_diff event in snapshot sync")
	}
	if !gotGameState {
		t.Fatal("expected final authoritative game_state after state_diff")
	}
	if diffPayload.RoomID != room.ID || diffPayload.MatchID != room.MatchID {
		t.Fatalf("state_diff identifiers mismatch: room=%s match=%s", diffPayload.RoomID, diffPayload.MatchID)
	}
	if diffPayload.FromVersion != 1 || diffPayload.ToVersion != syncPayload.ToVersion {
		t.Fatalf("state_diff version bounds mismatch: from=%d to=%d syncTo=%d", diffPayload.FromVersion, diffPayload.ToVersion, syncPayload.ToVersion)
	}
	var patch struct {
		Version   int64  `json:"version"`
		TrumpSuit string `json:"trumpSuit"`
	}
	if err := json.Unmarshal(diffPayload.Patch, &patch); err != nil {
		t.Fatalf("decode state_diff patch: %v", err)
	}
	if patch.Version != diffPayload.ToVersion {
		t.Fatalf("state_diff patch version mismatch: patch=%d toVersion=%d", patch.Version, diffPayload.ToVersion)
	}
	if patch.TrumpSuit == "" {
		t.Fatal("expected state_diff patch to include full DTO fields (trumpSuit is empty)")
	}
}

func TestWSReconnectSnapshotCanSkipFinalGameStateForDiffCapableClient(t *testing.T) {
	env := newWSContractEnv(t, 30*time.Second)
	players := env.createUsers(t, 2)
	room, state := env.createInGameRoomWithMatch(t, "podkidnoy", 10, players, engine.GameConfig{
		DeckSize: 36,
		Mode:     "podkidnoy",
	})
	attackCardID, _, ok := pickAttackDefensePair(state, players[0].ID, players[1].ID)
	if !ok {
		t.Fatal("unable to find attack card for state_diff skip-final-state test")
	}

	conn1 := env.dialRoom(t, room.ID, players[0].ID)
	defer conn1.Close()
	conn2 := env.dialRoom(t, room.ID, players[1].ID)
	defer conn2.Close()

	sendClientEvent(t, conn1, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	sendClientEvent(t, conn2, ws.ClientEvent{
		Type: "join_room",
		Payload: map[string]interface{}{
			"roomId": room.ID,
		},
	})
	_ = waitForEventType(t, conn1, "game_state", 4*time.Second)
	_ = waitForEventType(t, conn2, "game_state", 4*time.Second)

	sendClientEvent(t, conn1, ws.ClientEvent{
		Type: "make_move",
		Payload: map[string]interface{}{
			"roomId": room.ID,
			"action": "attack_card",
			"cardId": attackCardID,
		},
	})

	waitDeadline := time.Now().Add(4 * time.Second)
	var latestVersion int64
	for time.Now().Before(waitDeadline) {
		current, err := env.gamesSvc.GetState(env.ctx, room.MatchID)
		if err == nil && current.Version >= 2 {
			latestVersion = current.Version
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if latestVersion < 2 {
		t.Fatalf("expected match version >=2, got %d", latestVersion)
	}

	replayKey := "ws:replay:" + room.MatchID
	if err := env.redis.Del(env.ctx, replayKey).Err(); err != nil {
		t.Fatalf("clear replay redis key: %v", err)
	}

	hubRestarted := ws.NewHub()
	defer hubRestarted.Drain(200 * time.Millisecond)
	authRestarted := auth.NewService(env.usersRepo, env.redis, env.jwtSecret, time.Hour, 24*time.Hour, time.Minute, "")
	limiterRestarted := ratelimit.NewService(env.redis)
	handlerRestarted := ws.NewHandler(authRestarted, env.roomsSvc, env.gamesSvc, nil, env.usersRepo, 300, true, hubRestarted, nil, limiterRestarted, env.redis, "*", true)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handlerRestarted.ServeWS)
	serverRestarted := httptest.NewServer(mux)
	defer serverRestarted.Close()

	token := mustIssueJWT(t, env.jwtSecret, players[0].ID)
	wsURL := strings.Replace(serverRestarted.URL, "http://", "ws://", 1) + "/ws"
	connRestarted := dialRoomURLWithToken(t, wsURL, token, room.ID)
	defer connRestarted.Close()

	sendClientEvent(t, connRestarted, ws.ClientEvent{
		Type: "reconnect",
		Payload: map[string]interface{}{
			"roomId":            room.ID,
			"lastKnownVersion":  float64(1),
			"lastKnownMatchId":  room.MatchID,
			"supportsStateDiff": true,
		},
	})

	type stateSyncPayload struct {
		Mode        string `json:"mode"`
		FromVersion int64  `json:"fromVersion"`
		ToVersion   int64  `json:"toVersion"`
		ReplayCount int    `json:"replayCount"`
	}
	type stateDiffPayload struct {
		RoomID      string          `json:"roomId"`
		MatchID     string          `json:"matchId"`
		FromVersion int64           `json:"fromVersion"`
		ToVersion   int64           `json:"toVersion"`
		Patch       json.RawMessage `json:"patch"`
	}

	var (
		gotSync      bool
		gotStateDiff bool
		syncPayload  stateSyncPayload
		diffPayload  stateDiffPayload
	)

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		evt := readNextServerEvent(t, connRestarted, deadline)
		switch evt.Type {
		case "state_sync":
			if err := json.Unmarshal(evt.Payload, &syncPayload); err != nil {
				t.Fatalf("decode state_sync payload: %v", err)
			}
			gotSync = true
		case "state_diff":
			if err := json.Unmarshal(evt.Payload, &diffPayload); err != nil {
				t.Fatalf("decode state_diff payload: %v", err)
			}
			gotStateDiff = true
		case "game_state":
			t.Fatalf("unexpected final game_state in diff-capable skip mode: %s", string(evt.Payload))
		}
		if gotSync && gotStateDiff {
			break
		}
	}

	if !gotSync {
		t.Fatal("expected state_sync")
	}
	if !gotStateDiff {
		t.Fatal("expected state_diff")
	}
	if syncPayload.Mode != "snapshot" {
		t.Fatalf("expected snapshot mode when replay unavailable, got %s", syncPayload.Mode)
	}
	if syncPayload.ReplayCount != 0 {
		t.Fatalf("expected replayCount=0 in snapshot without replay, got %d", syncPayload.ReplayCount)
	}
	if diffPayload.RoomID != room.ID || diffPayload.MatchID != room.MatchID {
		t.Fatalf("state_diff identifiers mismatch: room=%s match=%s", diffPayload.RoomID, diffPayload.MatchID)
	}
	if diffPayload.FromVersion != 1 || diffPayload.ToVersion != syncPayload.ToVersion {
		t.Fatalf("state_diff version bounds mismatch: from=%d to=%d syncTo=%d", diffPayload.FromVersion, diffPayload.ToVersion, syncPayload.ToVersion)
	}

	var patch struct {
		Version   int64  `json:"version"`
		TrumpSuit string `json:"trumpSuit"`
	}
	if err := json.Unmarshal(diffPayload.Patch, &patch); err != nil {
		t.Fatalf("decode state_diff patch: %v", err)
	}
	if patch.Version != diffPayload.ToVersion {
		t.Fatalf("state_diff patch version mismatch: patch=%d toVersion=%d", patch.Version, diffPayload.ToVersion)
	}
	if patch.TrumpSuit == "" {
		t.Fatal("expected state_diff patch to include full DTO fields (trumpSuit is empty)")
	}

	if evt, ok, err := tryReadServerEvent(connRestarted, 300*time.Millisecond); err != nil {
		t.Fatalf("read after state_diff-only sync: %v", err)
	} else if ok && evt.Type == "game_state" {
		t.Fatalf("unexpected trailing game_state after state_diff-only sync: %s", string(evt.Payload))
	}
}
