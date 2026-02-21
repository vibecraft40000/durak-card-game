package integration

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

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
	if room.MatchID == "" {
		t.Fatal("match should be started after both ready")
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
		_, errs[0] = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, cardID)
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, cardID)
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
	_, err := pg.Exec(ctx, `TRUNCATE transactions, match_players, matches, users, game_history RESTART IDENTITY CASCADE`)
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
			_, _ = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, cardID)
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
	next, err := gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, cardID)
	if err != nil {
		t.Fatalf("apply after reconnect: %v", err)
	}
	if next.Status != engine.StatusPlaying && next.Status != engine.StatusFinished {
		t.Errorf("expected playing or finished, got %s", next.Status)
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
			_, _ = gamesSvc.Apply(ctx, matchID, "p1", engine.ActionAttack, cardID)
		}()
	}
	wg.Wait()
	// No panic or data race (run with -race). One goroutine wins, others get match finished/locked.
}
