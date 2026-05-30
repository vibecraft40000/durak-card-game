package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"durakonline/backend/internal/gameresults"
	"durakonline/backend/internal/games"
	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/rooms"
	"durakonline/backend/internal/transactions"
	"durakonline/backend/internal/users"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func TestMatchLifecycle(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	usersRepo := users.NewRepository(pg)
	txRepo := transactions.NewRepository(pg)
	walletSvc := wallet.NewService(pg, txRepo)
	gamesSvc := games.NewService(pg, redisClient, 30*time.Second, time.Hour)
	roomsRepo := rooms.NewRepository(redisClient)
	roomsSvc := rooms.NewService(roomsRepo, gamesSvc, walletSvc, 300, false)

	u1, _ := usersRepo.GetOrCreateByTelegram(ctx, 101, "u1", "User", "One", "")
	u2, _ := usersRepo.GetOrCreateByTelegram(ctx, 102, "u2", "User", "Two", "")
	_, _ = txRepo.Add(ctx, transactions.Transaction{UserID: u1.ID, Type: transactions.TypeDeposit, Amount: 100, Status: transactions.StatusConfirmed})
	_, _ = txRepo.Add(ctx, transactions.Transaction{UserID: u2.ID, Type: transactions.TypeDeposit, Amount: 100, Status: transactions.StatusConfirmed})

	room, err := roomsSvc.Create(ctx, "table", 10, 2, 36, "Подкидной", u1.ID)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := roomsSvc.Join(ctx, room.ID, u2.ID); err != nil {
		t.Fatalf("join room: %v", err)
	}
	if _, err := roomsSvc.Ready(ctx, room.ID, u1.ID); err != nil {
		t.Fatalf("ready u1: %v", err)
	}
	room, err = roomsSvc.Ready(ctx, room.ID, u2.ID)
	if err != nil {
		t.Fatalf("ready u2: %v", err)
	}
	if room.Status != rooms.StatusAwaitingStakeConfirm {
		t.Fatalf("expected awaiting stake confirm, got %s", room.Status)
	}
	if room.MatchID != "" {
		t.Fatal("match should not start before stake confirmation")
	}
	if _, err := roomsSvc.ConfirmStake(ctx, room.ID, u1.ID); err != nil {
		t.Fatalf("confirm stake u1: %v", err)
	}
	room, err = roomsSvc.ConfirmStake(ctx, room.ID, u2.ID)
	if err != nil {
		t.Fatalf("confirm stake u2: %v", err)
	}
	if room.MatchID == "" {
		t.Fatal("match should be started after stake confirmations")
	}
	if _, err := gamesSvc.GetState(ctx, room.MatchID); err != nil {
		t.Fatalf("state should exist in redis: %v", err)
	}
}

func TestDoubleMoveProtection(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	gamesSvc := games.NewService(pg, redisClient, 30*time.Second, time.Hour)
	matchID := uuid.NewString()
	state, err := gamesSvc.StartMatch(ctx, matchID, 10, "classic", []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("start match: %v", err)
	}
	cardID := state.Hands["p1"][0].ID

	var wg sync.WaitGroup
	wg.Add(2)
	errs := make([]error, 2)
	go func() {
		defer wg.Done()
		_, _, errs[0] = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, cardID, nil, "")
	}()
	go func() {
		defer wg.Done()
		_, _, errs[1] = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, cardID, nil, "")
	}()
	wg.Wait()

	if errs[0] == nil && errs[1] == nil {
		t.Fatal("one of concurrent moves must fail")
	}
}

func TestSettlementIdempotency(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	usersRepo := users.NewRepository(pg)
	txRepo := transactions.NewRepository(pg)
	walletSvc := wallet.NewService(pg, txRepo)
	u1, _ := usersRepo.GetOrCreateByTelegram(ctx, 201, "winner", "Winner", "Test", "")
	matchID := uuid.NewString()

	if err := walletSvc.SettleWin(ctx, u1.ID, matchID, 20, 300); err != nil {
		t.Fatalf("first settle: %v", err)
	}
	if err := walletSvc.SettleWin(ctx, u1.ID, matchID, 20, 300); err != nil {
		t.Fatalf("second settle (idempotent): %v", err)
	}

	var cnt int
	err := pg.QueryRow(ctx, `SELECT COUNT(*) FROM transactions WHERE match_id=$1 AND user_id=$2 AND type='win'`, matchID, u1.ID).Scan(&cnt)
	if err != nil {
		t.Fatalf("count win tx: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected exactly one win tx, got %d", cnt)
	}
	err = pg.QueryRow(ctx, `SELECT COUNT(*) FROM platform_fees WHERE match_id=$1`, matchID).Scan(&cnt)
	if err != nil {
		t.Fatalf("count platform fee rows: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected exactly one platform fee row, got %d", cnt)
	}
}

func TestSettleIfFinishedWritesPlatformFeeAudit(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	usersRepo := users.NewRepository(pg)
	txRepo := transactions.NewRepository(pg)
	walletSvc := wallet.NewService(pg, txRepo)
	gameResultsRepo := gameresults.NewRepository(pg)

	u1, _ := usersRepo.GetOrCreateByTelegram(ctx, 301, "winner", "Winner", "Audit", "")
	u2, _ := usersRepo.GetOrCreateByTelegram(ctx, 302, "loser", "Loser", "Audit", "")
	matchID := uuid.NewString()
	if _, err := pg.Exec(ctx, `
INSERT INTO matches (id, status, stake, mode, created_at, finished_at)
VALUES ($1, 'finished', $2, 'classic', NOW(), NOW())`, matchID, 50); err != nil {
		t.Fatalf("insert match fixture: %v", err)
	}
	state := engine.GameState{
		MatchID:       matchID,
		Status:        engine.StatusFinished,
		WinnerPlayer:  u1.ID,
		WinnerPlayers: []string{u1.ID},
		FinishGroups:  [][]string{{u1.ID}, {u2.ID}},
		PlayerOrder:   []string{u1.ID, u2.ID},
	}

	info, err := games.SettleIfFinished(ctx, pg, redisClient, walletSvc, gameResultsRepo, state, 50, 300)
	if err != nil {
		t.Fatalf("settle finished match: %v", err)
	}
	if info == nil {
		t.Fatal("expected payout info")
	}
	if info.Commission != 3 {
		t.Fatalf("expected commission 3, got %.2f", info.Commission)
	}

	var (
		commissionAmount float64
		grossPot         float64
		commissionBps    int
	)
	err = pg.QueryRow(ctx, `
SELECT commission_amount, gross_pot, commission_bps
FROM platform_fees
WHERE match_id = $1`, matchID).Scan(&commissionAmount, &grossPot, &commissionBps)
	if err != nil {
		t.Fatalf("select platform fee row: %v", err)
	}
	if commissionAmount != 3 {
		t.Fatalf("expected platform fee amount 3, got %.2f", commissionAmount)
	}
	if grossPot != 100 {
		t.Fatalf("expected gross pot 100, got %.2f", grossPot)
	}
	if commissionBps != 300 {
		t.Fatalf("expected commission bps 300, got %d", commissionBps)
	}
}

func TestPendingStartReconcileCompensatesHoldAndAllowsRetry(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	usersRepo := users.NewRepository(pg)
	txRepo := transactions.NewRepository(pg)
	walletSvc := wallet.NewService(pg, txRepo)
	gamesSvc := games.NewService(pg, redisClient, 30*time.Second, time.Hour)
	roomsRepo := rooms.NewRepository(redisClient)
	roomsSvc := rooms.NewService(roomsRepo, gamesSvc, walletSvc, 300, false)

	u1, _ := usersRepo.GetOrCreateByTelegram(ctx, 401, "reco1", "Reco", "One", "")
	u2, _ := usersRepo.GetOrCreateByTelegram(ctx, 402, "reco2", "Reco", "Two", "")
	_, _ = txRepo.Add(ctx, transactions.Transaction{UserID: u1.ID, Type: transactions.TypeDeposit, Amount: 100, Status: transactions.StatusConfirmed})
	_, _ = txRepo.Add(ctx, transactions.Transaction{UserID: u2.ID, Type: transactions.TypeDeposit, Amount: 100, Status: transactions.StatusConfirmed})

	room, err := roomsSvc.Create(ctx, "reconcile-room", 10, 2, 36, "classic", u1.ID)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	room, err = roomsSvc.Join(ctx, room.ID, u2.ID)
	if err != nil {
		t.Fatalf("join room: %v", err)
	}
	room.Status = rooms.StatusAwaitingStakeConfirm
	room.StakeConfirmDeadline = time.Now().UTC().Add(time.Minute).UnixMilli()
	if err := roomsRepo.Save(ctx, room); err != nil {
		t.Fatalf("save room: %v", err)
	}
	if err := roomsRepo.AddStakeConfirmedUser(ctx, room.ID, u1.ID); err != nil {
		t.Fatalf("confirm u1: %v", err)
	}
	if err := roomsRepo.AddStakeConfirmedUser(ctx, room.ID, u2.ID); err != nil {
		t.Fatalf("confirm u2: %v", err)
	}

	pendingMatchID, err := roomsRepo.GetOrCreatePendingStartMatchID(ctx, room.ID)
	if err != nil {
		t.Fatalf("pending match id: %v", err)
	}
	if err := walletSvc.HoldBet(ctx, u1.ID, pendingMatchID, room.Stake); err != nil {
		t.Fatalf("hold partial stake: %v", err)
	}

	if balance, err := walletSvc.Balance(ctx, u1.ID); err != nil || balance != 90 {
		t.Fatalf("expected balance 90 after partial hold, got balance=%v err=%v", balance, err)
	}

	if reconciled := roomsSvc.ReconcilePendingStarts(ctx); reconciled != 1 {
		t.Fatalf("expected one reconciled pending start, got %d", reconciled)
	}

	if balance, err := walletSvc.Balance(ctx, u1.ID); err != nil || balance != 100 {
		t.Fatalf("expected balance restored to 100 after reconcile, got balance=%v err=%v", balance, err)
	}

	var holdCount, releaseCount int
	if err := pg.QueryRow(ctx, `
SELECT COUNT(*) FROM transactions WHERE user_id=$1 AND match_id=$2 AND type='bet_hold'`,
		u1.ID, pendingMatchID,
	).Scan(&holdCount); err != nil {
		t.Fatalf("count bet_hold: %v", err)
	}
	if err := pg.QueryRow(ctx, `
SELECT COUNT(*) FROM transactions WHERE user_id=$1 AND match_id=$2 AND type='bet_hold_release'`,
		u1.ID, pendingMatchID,
	).Scan(&releaseCount); err != nil {
		t.Fatalf("count bet_hold_release: %v", err)
	}
	if holdCount != 1 || releaseCount != 1 {
		t.Fatalf("expected hold/release audit trail 1/1, got hold=%d release=%d", holdCount, releaseCount)
	}

	startedRoom, err := roomsSvc.StartGame(ctx, room.ID, u1.ID)
	if err != nil {
		t.Fatalf("retry start after reconcile: %v", err)
	}
	if startedRoom.MatchID == "" {
		t.Fatal("expected match to start after reconcile")
	}
	if _, err := gamesSvc.GetState(ctx, startedRoom.MatchID); err != nil {
		t.Fatalf("expected started match state: %v", err)
	}

	if balance, err := walletSvc.Balance(ctx, u1.ID); err != nil || balance != 90 {
		t.Fatalf("expected u1 balance 90 after retry start, got balance=%v err=%v", balance, err)
	}
	if balance, err := walletSvc.Balance(ctx, u2.ID); err != nil || balance != 90 {
		t.Fatalf("expected u2 balance 90 after retry start, got balance=%v err=%v", balance, err)
	}
}

func TestConfirmStakeIsIdempotentAfterMatchStart(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	usersRepo := users.NewRepository(pg)
	txRepo := transactions.NewRepository(pg)
	walletSvc := wallet.NewService(pg, txRepo)
	gamesSvc := games.NewService(pg, redisClient, 30*time.Second, time.Hour)
	roomsRepo := rooms.NewRepository(redisClient)
	roomsSvc := rooms.NewService(roomsRepo, gamesSvc, walletSvc, 300, false)

	u1, _ := usersRepo.GetOrCreateByTelegram(ctx, 451, "stake1", "Stake", "One", "")
	u2, _ := usersRepo.GetOrCreateByTelegram(ctx, 452, "stake2", "Stake", "Two", "")
	_, _ = txRepo.Add(ctx, transactions.Transaction{UserID: u1.ID, Type: transactions.TypeDeposit, Amount: 100, Status: transactions.StatusConfirmed})
	_, _ = txRepo.Add(ctx, transactions.Transaction{UserID: u2.ID, Type: transactions.TypeDeposit, Amount: 100, Status: transactions.StatusConfirmed})

	room, err := roomsSvc.Create(ctx, "stake-room", 10, 2, 36, "classic", u1.ID)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := roomsSvc.Join(ctx, room.ID, u2.ID); err != nil {
		t.Fatalf("join room: %v", err)
	}
	if _, err := roomsSvc.Ready(ctx, room.ID, u1.ID); err != nil {
		t.Fatalf("ready u1: %v", err)
	}
	room, err = roomsSvc.Ready(ctx, room.ID, u2.ID)
	if err != nil {
		t.Fatalf("ready u2: %v", err)
	}
	if room.Status != rooms.StatusAwaitingStakeConfirm {
		t.Fatalf("expected awaiting stake confirm, got %s", room.Status)
	}

	if _, err := roomsSvc.ConfirmStake(ctx, room.ID, u1.ID); err != nil {
		t.Fatalf("confirm stake u1: %v", err)
	}
	startedRoom, err := roomsSvc.ConfirmStake(ctx, room.ID, u2.ID)
	if err != nil {
		t.Fatalf("confirm stake u2: %v", err)
	}
	if startedRoom.Status != rooms.StatusInGame || startedRoom.MatchID == "" {
		t.Fatalf("expected room to be in_game with match id, got status=%s match=%q", startedRoom.Status, startedRoom.MatchID)
	}

	duplicateConfirmRoom, err := roomsSvc.ConfirmStake(ctx, room.ID, u2.ID)
	if err != nil {
		t.Fatalf("duplicate confirm stake u2: %v", err)
	}
	if duplicateConfirmRoom.Status != rooms.StatusInGame {
		t.Fatalf("expected duplicate confirm to keep in_game, got %s", duplicateConfirmRoom.Status)
	}
	if duplicateConfirmRoom.MatchID != startedRoom.MatchID {
		t.Fatalf("expected duplicate confirm to keep same match id, got %q want %q", duplicateConfirmRoom.MatchID, startedRoom.MatchID)
	}

	var holdCount int
	if err := pg.QueryRow(ctx, `
SELECT COUNT(*) FROM transactions WHERE match_id=$1 AND type='bet_hold'`,
		startedRoom.MatchID,
	).Scan(&holdCount); err != nil {
		t.Fatalf("count hold tx: %v", err)
	}
	if holdCount != 2 {
		t.Fatalf("expected exactly 2 hold tx rows, got %d", holdCount)
	}
}

func integrationDeps(t *testing.T) (*pgxpool.Pool, *redis.Client) {
	t.Helper()
	pgDSN := os.Getenv("INTEGRATION_PG_DSN")
	redisURL := os.Getenv("INTEGRATION_REDIS_URL")
	if pgDSN == "" || redisURL == "" {
		t.Skip("set INTEGRATION_PG_DSN and INTEGRATION_REDIS_URL")
	}
	pg, err := storage.NewPostgresPool(context.Background(), pgDSN)
	if err != nil {
		t.Fatalf("pg connect: %v", err)
	}
	redisClient, err := storage.NewRedisClient(redisURL)
	if err != nil {
		t.Fatalf("redis connect: %v", err)
	}
	return pg, redisClient
}

func resetData(t *testing.T, ctx context.Context, pg *pgxpool.Pool) {
	t.Helper()
	_, err := pg.Exec(ctx, `TRUNCATE platform_fees, transactions, game_results, match_players, matches, users, game_history RESTART IDENTITY CASCADE`)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("truncate tables: %v", err)
	}
}

// TestReconnectVsMakeMoveRace exercises concurrent ClearDisconnected and Apply (make_move).
// Both may run when a player reconnects while the other is making a move.
func TestReconnectVsMakeMoveRace(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	gamesSvc := games.NewService(pg, redisClient, 30*time.Second, time.Hour)
	matchID := uuid.NewString()
	state, err := gamesSvc.StartMatch(ctx, matchID, 10, "classic", []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("start match: %v", err)
	}
	cardID := state.Hands["p1"][0].ID

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _, _ = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, cardID, nil, "")
		}()
		go func() {
			defer wg.Done()
			_ = gamesSvc.MarkDisconnected(ctx, matchID, "p2")
			_ = gamesSvc.ClearDisconnected(ctx, matchID, "p2")
		}()
	}
	wg.Wait()
	// No panic or data race (run with -race)
}

// TestReconnectFlow verifies: disconnect -> reconnect -> continue move.
// MarkDisconnected, ClearDisconnected, then Apply must succeed.
func TestReconnectFlow(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	gamesSvc := games.NewService(pg, redisClient, 30*time.Second, time.Hour)
	matchID := uuid.NewString()
	state, err := gamesSvc.StartMatch(ctx, matchID, 10, "classic", []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("start match: %v", err)
	}
	cardID := state.Hands["p1"][0].ID

	if err := gamesSvc.MarkDisconnected(ctx, matchID, "p2"); err != nil {
		t.Fatalf("mark disconnected: %v", err)
	}
	if err := gamesSvc.ClearDisconnected(ctx, matchID, "p2"); err != nil {
		t.Fatalf("clear disconnected: %v", err)
	}
	next, _, err := gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, cardID, nil, "")
	if err != nil {
		t.Fatalf("apply after reconnect: %v", err)
	}
	if next.Status != engine.StatusPlaying && next.Status != engine.StatusFinished {
		t.Errorf("expected playing or finished, got %s", next.Status)
	}
}

// TestShulerReportWindowAllowsReportAfterReconnect verifies that report remains
// available after a reconnect/state-resync while window is still open.
func TestShulerReportWindowAllowsReportAfterReconnect(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	gamesSvc := games.NewService(pg, redisClient, 30*time.Second, time.Hour)
	matchID := uuid.NewString()
	state, err := gamesSvc.StartMatchWithConfig(ctx, matchID, 10, engine.GameConfig{
		DeckSize:       36,
		Mode:           "podkidnoy",
		ShulerEnabled:  true,
		ShulerPlayerID: "p2",
	}, []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("start match with shuler config: %v", err)
	}

	attackCardID := state.Hands["p1"][0].ID
	state, _, err = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, attackCardID, nil, "")
	if err != nil {
		t.Fatalf("attack before shuler play: %v", err)
	}
	shulerCardID := state.Hands["p2"][0].ID
	state, _, err = gamesSvc.Apply(ctx, matchID, "p2", engine.ActionShulerPlay, shulerCardID, nil, "")
	if err != nil {
		t.Fatalf("shuler play: %v", err)
	}
	if state.ShulerWindowUntil.IsZero() {
		t.Fatal("expected shuler window to be opened")
	}

	// Simulate reconnect markers, then state resync.
	if err := gamesSvc.MarkDisconnected(ctx, matchID, "p1"); err != nil {
		t.Fatalf("mark disconnected: %v", err)
	}
	if err := gamesSvc.ClearDisconnected(ctx, matchID, "p1"); err != nil {
		t.Fatalf("clear disconnected: %v", err)
	}
	resynced, err := gamesSvc.GetState(ctx, matchID)
	if err != nil {
		t.Fatalf("get state after reconnect: %v", err)
	}
	if resynced.ShulerWindowUntil.IsZero() {
		t.Fatal("expected shuler window timestamp in resynced state")
	}
	if time.Now().UTC().After(resynced.ShulerWindowUntil) {
		t.Fatalf("window expired too early in test setup: now=%s window=%s", time.Now().UTC(), resynced.ShulerWindowUntil)
	}

	// Non-shuler player may still report inside window.
	resynced, _, err = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionShulerReport, "", nil, "")
	if err != nil {
		t.Fatalf("shuler report inside window: %v", err)
	}
	if !resynced.ShulerDetected {
		t.Fatal("expected shuler to be detected after report")
	}
}

// TestShulerReportWindowExpiresAcrossReconnect verifies boundary behavior:
// after reconnect/state-resync and window expiry, report is denied.
func TestShulerReportWindowExpiresAcrossReconnect(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	gamesSvc := games.NewService(pg, redisClient, 30*time.Second, time.Hour)
	matchID := uuid.NewString()
	state, err := gamesSvc.StartMatchWithConfig(ctx, matchID, 10, engine.GameConfig{
		DeckSize:       36,
		Mode:           "podkidnoy",
		ShulerEnabled:  true,
		ShulerPlayerID: "p2",
	}, []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("start match with shuler config: %v", err)
	}

	attackCardID := state.Hands["p1"][0].ID
	state, _, err = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, attackCardID, nil, "")
	if err != nil {
		t.Fatalf("attack before shuler play: %v", err)
	}
	shulerCardID := state.Hands["p2"][0].ID
	state, _, err = gamesSvc.Apply(ctx, matchID, "p2", engine.ActionShulerPlay, shulerCardID, nil, "")
	if err != nil {
		t.Fatalf("shuler play: %v", err)
	}
	if state.ShulerWindowUntil.IsZero() {
		t.Fatal("expected shuler window to be opened")
	}

	// Simulate reconnect markers, then resync state from storage.
	if err := gamesSvc.MarkDisconnected(ctx, matchID, "p1"); err != nil {
		t.Fatalf("mark disconnected: %v", err)
	}
	if err := gamesSvc.ClearDisconnected(ctx, matchID, "p1"); err != nil {
		t.Fatalf("clear disconnected: %v", err)
	}
	resynced, err := gamesSvc.GetState(ctx, matchID)
	if err != nil {
		t.Fatalf("get state after reconnect: %v", err)
	}

	waitUntil := resynced.ShulerWindowUntil.Add(120 * time.Millisecond)
	for time.Now().UTC().Before(waitUntil) {
		time.Sleep(20 * time.Millisecond)
	}

	_, _, err = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionShulerReport, "", nil, "")
	if !errors.Is(err, engine.ErrCannotReportShuler) {
		t.Fatalf("expected ErrCannotReportShuler after window expiry, got %v", err)
	}
}

// TestTimeoutVersionMismatchRetryAroundThrowWindow verifies websocket-like flow:
// stale expectedVersion after timeout yields ErrVersionMismatch, and retry with
// fresh version yields business validation error for already-closed throw window.
func TestTimeoutVersionMismatchRetryAroundThrowWindow(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	gamesSvc := games.NewService(pg, redisClient, 300*time.Millisecond, time.Hour)

	const maxAttempts = 30
	var (
		matchID      string
		throwCardID  string
		staleVersion int64
		prepared     bool
	)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		candidateMatchID := uuid.NewString()
		state, err := gamesSvc.StartMatch(ctx, candidateMatchID, 10, "classic", []string{"p1", "p2", "p3"})
		if err != nil {
			t.Fatalf("start match: %v", err)
		}

		attackCardID, defendCardID, throwCandidateID, ok := pickThrowWindowScenario(state)
		if !ok {
			continue
		}

		state, _, err = gamesSvc.Apply(ctx, candidateMatchID, "p1", engine.ActionAttack, attackCardID, nil, "")
		if err != nil {
			continue
		}
		state, _, err = gamesSvc.Apply(ctx, candidateMatchID, "p2", engine.ActionDefend, defendCardID, nil, "")
		if err != nil {
			continue
		}
		if state.TurnState != engine.TurnAttack || state.TurnPlayerID != "p1" || len(state.TableCards) == 0 || len(state.TableCards)%2 != 0 {
			continue
		}

		matchID = candidateMatchID
		throwCardID = throwCandidateID
		staleVersion = state.Version
		prepared = true
		break
	}

	if !prepared {
		t.Fatalf("unable to prepare throw window scenario in %d attempts", maxAttempts)
	}

	timeoutApplied := false
	waitDeadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(waitDeadline) {
		results := gamesSvc.HandleTimeouts(ctx)
		for _, result := range results {
			if result.MatchID == matchID {
				timeoutApplied = true
				break
			}
		}
		if timeoutApplied {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if !timeoutApplied {
		t.Fatal("expected timeout to be applied for prepared match")
	}

	_, _, err := gamesSvc.Apply(ctx, matchID, "p3", engine.ActionThrow, throwCardID, &staleVersion, "")
	if !errors.Is(err, games.ErrVersionMismatch) {
		t.Fatalf("expected ErrVersionMismatch for stale expectedVersion, got %v", err)
	}

	latest, err := gamesSvc.GetState(ctx, matchID)
	if err != nil {
		t.Fatalf("get latest state: %v", err)
	}
	currentVersion := latest.Version
	_, _, err = gamesSvc.Apply(ctx, matchID, "p3", engine.ActionThrow, throwCardID, &currentVersion, "")
	if !errors.Is(err, engine.ErrThrowDenied) {
		t.Fatalf("expected ErrThrowDenied on retry with fresh version, got %v", err)
	}
}

func pickThrowWindowScenario(state engine.GameState) (attackCardID, defendCardID, throwCardID string, ok bool) {
	attackerHand := state.Hands["p1"]
	defenderHand := state.Hands["p2"]
	throwerHand := state.Hands["p3"]

	for _, attackCard := range attackerHand {
		defendCard, canDefend := pickDefenseCard(defenderHand, attackCard, state.Trump)
		if !canDefend {
			continue
		}
		for _, throwCard := range throwerHand {
			if throwCard.Rank == attackCard.Rank || throwCard.Rank == defendCard.Rank {
				return attackCard.ID, defendCard.ID, throwCard.ID, true
			}
		}
	}
	return "", "", "", false
}

func pickTranslateScenario(state engine.GameState, attackerID, defenderID string) (attackCardID, translateCardID string, ok bool) {
	attackerHand := state.Hands[attackerID]
	defenderHand := state.Hands[defenderID]
	for _, attackCard := range attackerHand {
		for _, translateCard := range defenderHand {
			if translateCard.Rank == attackCard.Rank {
				return attackCard.ID, translateCard.ID, true
			}
		}
	}
	return "", "", false
}

func pickDefenseCard(hand []engine.Card, attack engine.Card, trump engine.Suit) (engine.Card, bool) {
	for _, candidate := range hand {
		if cardBeats(attack, candidate, trump) {
			return candidate, true
		}
	}
	return engine.Card{}, false
}

func cardBeats(attack engine.Card, defend engine.Card, trump engine.Suit) bool {
	if defend.Suit == attack.Suit {
		return rankWeightForTest(defend.Rank) > rankWeightForTest(attack.Rank)
	}
	return defend.Suit == trump && attack.Suit != trump
}

func rankWeightForTest(rank string) int {
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

// TestDisconnectVsAbandonRace exercises concurrent ForceAbandon and Apply.
// HandleDisconnectTimeouts may call ForceAbandon while a move is in flight.
func TestDisconnectVsAbandonRace(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	gamesSvc := games.NewService(pg, redisClient, 30*time.Second, time.Hour)
	matchID := uuid.NewString()
	state, err := gamesSvc.StartMatch(ctx, matchID, 10, "classic", []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("start match: %v", err)
	}
	cardID := state.Hands["p1"][0].ID

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = gamesSvc.ForceAbandon(ctx, matchID, "p2")
		}()
		go func() {
			defer wg.Done()
			_, _, _ = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, cardID, nil, "")
		}()
	}
	wg.Wait()
	// No panic or data race (run with -race). One goroutine wins, others get match finished/locked.
}

func TestDisconnectPolicyBotTakeoverKeepsMatchActive(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	gamesSvc := games.NewService(pg, redisClient, 30*time.Second, time.Hour)
	gamesSvc.SetDisconnectPolicy(games.DisconnectPolicyBotTakeover)
	matchID := uuid.NewString()
	_, err := gamesSvc.StartMatch(ctx, matchID, 10, "classic", []string{"p1", "p2"})
	if err != nil {
		t.Fatalf("start match: %v", err)
	}

	staleTS := strconv.FormatInt(time.Now().Add(-70*time.Second).Unix(), 10)
	disconnectedMarker := fmt.Sprintf("match:disconnected:%s:%s", matchID, "p2")
	if err := redisClient.Set(ctx, disconnectedMarker, staleTS, 2*time.Minute).Err(); err != nil {
		t.Fatalf("set disconnected marker: %v", err)
	}

	results := gamesSvc.HandleDisconnectTimeouts(ctx)
	if len(results) == 0 {
		t.Fatal("expected disconnect resolution result")
	}
	result := results[0]
	if result.Kind != games.DisconnectResolutionBotTakeover {
		t.Fatalf("expected bot_takeover resolution, got %s", result.Kind)
	}
	if result.PlayerID != "p2" {
		t.Fatalf("expected player p2 to be resolved, got %s", result.PlayerID)
	}

	state, err := gamesSvc.GetState(ctx, matchID)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if state.Status != engine.StatusPlaying {
		t.Fatalf("expected playing status after bot takeover, got %s", state.Status)
	}
}

func TestTranslateRoundTimeoutReconnectRaceVersionMismatch(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()
	resetData(t, ctx, pg)

	gamesSvc := games.NewService(pg, redisClient, 300*time.Millisecond, time.Hour)

	const maxAttempts = 40
	var (
		matchID      string
		staleVersion int64
		prepared     bool
	)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		candidateMatchID := uuid.NewString()
		state, err := gamesSvc.StartMatchWithConfig(ctx, candidateMatchID, 10, engine.GameConfig{
			DeckSize: 36,
			Mode:     "perevodnoy",
		}, []string{"p1", "p2", "p3"})
		if err != nil {
			t.Fatalf("start perevodnoy match: %v", err)
		}

		attackCardID, translateCardID, ok := pickTranslateScenario(state, "p1", "p2")
		if !ok {
			continue
		}
		state, _, err = gamesSvc.Apply(ctx, candidateMatchID, "p1", engine.ActionAttack, attackCardID, nil, "")
		if err != nil {
			continue
		}
		state, _, err = gamesSvc.Apply(ctx, candidateMatchID, "p2", engine.ActionTranslate, translateCardID, nil, "")
		if err != nil {
			continue
		}
		if state.TurnState != engine.TurnDefend || state.TurnPlayerID != "p3" || state.DefenderID != "p3" {
			continue
		}
		matchID = candidateMatchID
		staleVersion = state.Version
		prepared = true
		break
	}

	if !prepared {
		t.Fatalf("unable to prepare translate scenario in %d attempts", maxAttempts)
	}

	var (
		timeoutApplied bool
		mu             sync.Mutex
		wg             sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		waitDeadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(waitDeadline) {
			results := gamesSvc.HandleTimeouts(ctx)
			for _, result := range results {
				if result.MatchID == matchID {
					mu.Lock()
					timeoutApplied = true
					mu.Unlock()
					return
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 18; i++ {
			_ = gamesSvc.MarkDisconnected(ctx, matchID, "p3")
			_ = gamesSvc.ClearDisconnected(ctx, matchID, "p3")
			time.Sleep(12 * time.Millisecond)
		}
	}()
	wg.Wait()

	mu.Lock()
	applied := timeoutApplied
	mu.Unlock()
	if !applied {
		t.Fatal("expected timeout to be applied during translate reconnect race")
	}

	_, _, err := gamesSvc.Apply(ctx, matchID, "p1", engine.ActionTranslate, "", &staleVersion, "")
	if !errors.Is(err, games.ErrVersionMismatch) {
		t.Fatalf("expected ErrVersionMismatch on stale version after timeout, got %v", err)
	}

	latest, err := gamesSvc.GetState(ctx, matchID)
	if err != nil {
		t.Fatalf("get latest state: %v", err)
	}
	currentVersion := latest.Version
	_, _, err = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionTranslate, "", &currentVersion, "")
	if !errors.Is(err, engine.ErrTranslateDenied) && !errors.Is(err, engine.ErrInvalidTurn) {
		t.Fatalf("expected translated-round business denial on retry with fresh version, got %v", err)
	}
}
