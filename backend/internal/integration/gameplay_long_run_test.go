package integration

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"durakonline/backend/internal/gameresults"
	"durakonline/backend/internal/games"
	"durakonline/backend/internal/games/engine"
	"durakonline/backend/internal/rooms"
	"durakonline/backend/internal/transactions"
	"durakonline/backend/internal/users"
	"durakonline/backend/internal/wallet"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPublicBetaLongMatchSeriesMoneyInvariants(t *testing.T) {
	ctx := context.Background()
	pg, redisClient := integrationDeps(t)
	defer pg.Close()
	defer redisClient.Close()

	testCases := []struct {
		name           string
		players        int
		stakes         []float64
		matches        int
		persistHistory bool
	}{
		{name: "players_2", players: 2, stakes: []float64{1, 3, 10, 50}, matches: 50, persistHistory: true},
		{name: "players_3", players: 3, stakes: []float64{1, 3, 10, 25}, matches: 50},
		{name: "players_4", players: 4, stakes: []float64{0.01, 1, 5, 20}, matches: 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resetData(t, ctx, pg)

			usersRepo := users.NewRepository(pg)
			gameResultsRepo := gameresults.NewRepository(pg)
			txRepo := transactions.NewRepository(pg)
			walletSvc := wallet.NewService(pg, txRepo)
			gamesDB := pg
			if !tc.persistHistory {
				gamesDB = nil
			}
			gamesSvc := games.NewService(gamesDB, redisClient, 30*time.Second, time.Hour)
			roomsRepo := rooms.NewRepository(redisClient)
			roomsSvc := rooms.NewService(roomsRepo, gamesSvc, walletSvc, 300, false)

			seriesUsers := createSeriesUsers(t, ctx, usersRepo, tc.players)
			for _, user := range seriesUsers {
				if _, err := txRepo.Add(ctx, transactions.Transaction{
					UserID: user.ID,
					Type:   transactions.TypeDeposit,
					Amount: 5000,
					Status: transactions.StatusConfirmed,
				}); err != nil {
					t.Fatalf("seed deposit for %s: %v", user.ID, err)
				}
			}

			playerIDs := make([]string, 0, len(seriesUsers))
			for _, user := range seriesUsers {
				playerIDs = append(playerIDs, user.ID)
			}

			initialTotalCents := sumBalancesCents(t, ctx, walletSvc, playerIDs)
			var totalCommissionCents int64

			for matchIndex := 0; matchIndex < tc.matches; matchIndex++ {
				stake := tc.stakes[matchIndex%len(tc.stakes)]
				room, err := roomsSvc.Create(ctx, fmt.Sprintf("series-%s-%d", tc.name, matchIndex+1), stake, tc.players, 36, "podkidnoy", playerIDs[0])
				if err != nil {
					t.Fatalf("create room #%d: %v", matchIndex+1, err)
				}
				for _, playerID := range playerIDs[1:] {
					room, err = roomsSvc.Join(ctx, room.ID, playerID)
					if err != nil {
						t.Fatalf("join player %s for match #%d: %v", playerID, matchIndex+1, err)
					}
				}
				for _, playerID := range playerIDs {
					room, err = roomsSvc.Ready(ctx, room.ID, playerID)
					if err != nil && !errors.Is(err, rooms.ErrNeedOpponent) {
						t.Fatalf("ready player %s for match #%d: %v", playerID, matchIndex+1, err)
					}
				}
				if room.Status != rooms.StatusAwaitingStakeConfirm {
					t.Fatalf("expected awaiting stake confirm for match #%d, got %s", matchIndex+1, room.Status)
				}

				stateBeforeInvalidCheck := engine.GameState{}
				for _, playerID := range playerIDs {
					room, err = roomsSvc.ConfirmStake(ctx, room.ID, playerID)
					if err != nil {
						t.Fatalf("confirm stake player %s for match #%d: %v", playerID, matchIndex+1, err)
					}
				}
				if room.MatchID == "" || room.Status != rooms.StatusInGame {
					t.Fatalf("expected match #%d to start, got status=%s match=%q", matchIndex+1, room.Status, room.MatchID)
				}
				stateBeforeInvalidCheck, err = gamesSvc.GetState(ctx, room.MatchID)
				if err != nil {
					t.Fatalf("load match state #%d: %v", matchIndex+1, err)
				}

				if matchIndex == 0 {
					wrongPlayerID := firstNonTurnPlayer(stateBeforeInvalidCheck)
					if wrongPlayerID == "" {
						t.Fatalf("failed to find non-turn player for invalid move check")
					}
					hand := stateBeforeInvalidCheck.Hands[wrongPlayerID]
					if len(hand) == 0 {
						t.Fatalf("invalid move check requires hand cards for %s", wrongPlayerID)
					}
					if _, _, err := gamesSvc.Apply(ctx, room.MatchID, wrongPlayerID, engine.ActionAttack, hand[0].ID, nil, fmt.Sprintf("invalid-%s", room.MatchID)); !errors.Is(err, engine.ErrInvalidTurn) {
						t.Fatalf("expected invalid turn on live series match, got %v", err)
					}
				}

				finishedState, steps := autoplayMatchToFinish(t, ctx, gamesSvc, room.MatchID)
				if finishedState.Status != engine.StatusFinished {
					t.Fatalf("expected match #%d to finish, got %s after %d steps", matchIndex+1, finishedState.Status, steps)
				}
				ensureFinishedMatchRow(t, ctx, pg, finishedState.MatchID, stake, room.Mode)

				payoutInfo, err := games.SettleIfFinished(
					ctx,
					pg,
					redisClient,
					walletSvc,
					gameResultsRepo,
					finishedState,
					stake,
					300,
				)
				if err != nil {
					t.Fatalf("settle match #%d: %v", matchIndex+1, err)
				}
				if payoutInfo == nil {
					t.Fatalf("expected payout info for match #%d", matchIndex+1)
				}
				if _, err := games.SettleIfFinished(ctx, pg, redisClient, walletSvc, nil, finishedState, stake, 300); err != nil {
					t.Fatalf("duplicate settle match #%d: %v", matchIndex+1, err)
				}

				commissionCents := verifySuccessfulMatchMoneyInvariants(t, ctx, pg, room.MatchID, stake, tc.players)
				totalCommissionCents += commissionCents

				currentTotalCents := sumBalancesCents(t, ctx, walletSvc, playerIDs)
				expectedTotalCents := initialTotalCents - totalCommissionCents
				if currentTotalCents != expectedTotalCents {
					t.Fatalf(
						"balance conservation mismatch after match #%d: got=%d want=%d (players=%d)",
						matchIndex+1,
						currentTotalCents,
						expectedTotalCents,
						tc.players,
					)
				}
			}
		})
	}
}

func ensureFinishedMatchRow(t *testing.T, ctx context.Context, db *pgxpool.Pool, matchID string, stake float64, mode string) {
	t.Helper()
	if _, err := db.Exec(ctx, `
INSERT INTO matches (id, status, stake, mode, created_at, finished_at)
VALUES ($1, 'finished', $2, $3, NOW(), NOW())
ON CONFLICT (id) DO NOTHING`,
		matchID,
		stake,
		mode,
	); err != nil {
		t.Fatalf("ensure finished match row %s: %v", matchID, err)
	}
}

func createSeriesUsers(t *testing.T, ctx context.Context, usersRepo *users.Repository, count int) []users.User {
	t.Helper()
	out := make([]users.User, 0, count)
	baseTG := int64(990000 + count*1000)
	for i := 0; i < count; i++ {
		user, err := usersRepo.GetOrCreateByTelegram(
			ctx,
			baseTG+int64(i),
			fmt.Sprintf("series_%d_%d", count, i+1),
			"Series",
			fmt.Sprintf("P%d", i+1),
			"",
		)
		if err != nil {
			t.Fatalf("create series user %d: %v", i+1, err)
		}
		out = append(out, user)
	}
	return out
}

func autoplayMatchToFinish(t *testing.T, ctx context.Context, gamesSvc *games.Service, matchID string) (engine.GameState, int) {
	t.Helper()
	state, err := gamesSvc.GetState(ctx, matchID)
	if err != nil {
		t.Fatalf("load initial state %s: %v", matchID, err)
	}

	const maxSteps = 512
	for step := 0; step < maxSteps; step++ {
		if state.Status == engine.StatusFinished {
			return state, step
		}
		playerID, action, cardID, ok := pickAutoplayAction(state)
		if !ok {
			t.Fatalf("no valid autoplay action for match %s at version=%d turn=%s phase=%s", matchID, state.Version, state.TurnPlayerID, state.TurnState)
		}
		next, applied, err := gamesSvc.Apply(ctx, matchID, playerID, action, cardID, nil, fmt.Sprintf("%s-%d", matchID, step))
		if err != nil {
			t.Fatalf("autoplay apply failed match=%s player=%s action=%s card=%s version=%d: %v", matchID, playerID, action, cardID, state.Version, err)
		}
		if !applied {
			t.Fatalf("autoplay action unexpectedly treated as duplicate for match=%s step=%d", matchID, step)
		}
		state = next
	}

	t.Fatalf("autoplay exceeded %d steps for match %s", maxSteps, matchID)
	return engine.GameState{}, 0
}

func pickAutoplayAction(state engine.GameState) (string, engine.Action, string, bool) {
	if state.Status != engine.StatusPlaying {
		return "", "", "", false
	}

	candidates := make([]string, 0, len(state.PlayerOrder))
	if state.TurnPlayerID != "" {
		candidates = append(candidates, state.TurnPlayerID)
	}
	for _, playerID := range state.PlayerOrder {
		if playerID == state.TurnPlayerID {
			continue
		}
		candidates = append(candidates, playerID)
	}

	for _, playerID := range candidates {
		aff := engine.ViewAffordances(state, playerID)
		if !aff.CanAct {
			continue
		}
		if playerID == state.TurnPlayerID {
			switch {
			case aff.CanDefend && len(aff.DefendCardIDs) > 0:
				return playerID, engine.ActionDefend, aff.DefendCardIDs[0], true
			case aff.CanTake:
				return playerID, engine.ActionTake, "", true
			case aff.CanPass:
				return playerID, engine.ActionPass, "", true
			case aff.CanAttack && len(aff.AttackCardIDs) > 0:
				return playerID, engine.ActionAttack, aff.AttackCardIDs[0], true
			case aff.CanTranslate && len(aff.TranslateCardIDs) > 0:
				return playerID, engine.ActionTranslate, aff.TranslateCardIDs[0], true
			}
			continue
		}
		if aff.CanThrowIn && len(aff.ThrowInCardIDs) > 0 {
			return playerID, engine.ActionThrow, aff.ThrowInCardIDs[0], true
		}
	}

	return "", "", "", false
}

func firstNonTurnPlayer(state engine.GameState) string {
	for _, playerID := range state.PlayerOrder {
		if playerID != state.TurnPlayerID {
			return playerID
		}
	}
	return ""
}

func verifySuccessfulMatchMoneyInvariants(t *testing.T, ctx context.Context, db *pgxpool.Pool, matchID string, stake float64, players int) int64 {
	t.Helper()

	grossPotCents := invariantCents(stake * float64(players))
	var (
		holdCount        int
		holdSum          float64
		releaseCount     int
		releaseSum       float64
		winCount         int
		winSum           float64
		platformFeeRows  int
		platformFeeSum   float64
		grossPotStored   float64
		gameResultsCount int
		gameResultsGross float64
		gameResultsNet   float64
	)

	if err := db.QueryRow(ctx, `
SELECT COUNT(*), COALESCE(SUM(ABS(amount)), 0)
FROM transactions
WHERE match_id=$1 AND type='bet_hold'`, matchID).Scan(&holdCount, &holdSum); err != nil {
		t.Fatalf("hold summary for %s: %v", matchID, err)
	}
	if err := db.QueryRow(ctx, `
SELECT COUNT(*), COALESCE(SUM(amount), 0)
FROM transactions
WHERE match_id=$1 AND type='bet_hold_release'`, matchID).Scan(&releaseCount, &releaseSum); err != nil {
		t.Fatalf("release summary for %s: %v", matchID, err)
	}
	if err := db.QueryRow(ctx, `
SELECT COUNT(*), COALESCE(SUM(amount), 0)
FROM transactions
WHERE match_id=$1 AND type='win'`, matchID).Scan(&winCount, &winSum); err != nil {
		t.Fatalf("win summary for %s: %v", matchID, err)
	}
	if err := db.QueryRow(ctx, `
SELECT COUNT(*), COALESCE(SUM(commission_amount), 0), COALESCE(MAX(gross_pot), 0)
FROM platform_fees
WHERE match_id=$1`, matchID).Scan(&platformFeeRows, &platformFeeSum, &grossPotStored); err != nil {
		t.Fatalf("platform fee summary for %s: %v", matchID, err)
	}
	if err := db.QueryRow(ctx, `
SELECT COUNT(*), COALESCE(SUM(payout), 0), COALESCE(SUM(profit), 0)
FROM game_results
WHERE match_id=$1`, matchID).Scan(&gameResultsCount, &gameResultsGross, &gameResultsNet); err != nil {
		t.Fatalf("game results summary for %s: %v", matchID, err)
	}

	if holdCount != players {
		t.Fatalf("match %s: expected %d hold tx rows, got %d", matchID, players, holdCount)
	}
	if invariantCents(holdSum) != grossPotCents {
		t.Fatalf("match %s: hold sum mismatch got=%d want=%d", matchID, invariantCents(holdSum), grossPotCents)
	}
	if releaseCount != 0 || invariantCents(releaseSum) != 0 {
		t.Fatalf("match %s: successful match must not have releases, got count=%d sum=%.2f", matchID, releaseCount, releaseSum)
	}
	if platformFeeRows != 1 {
		t.Fatalf("match %s: expected one platform fee row, got %d", matchID, platformFeeRows)
	}
	if invariantCents(grossPotStored) != grossPotCents {
		t.Fatalf("match %s: stored gross pot mismatch got=%d want=%d", matchID, invariantCents(grossPotStored), grossPotCents)
	}
	if winCount <= 0 {
		t.Fatalf("match %s: expected at least one win tx row, got %d", matchID, winCount)
	}
	if gameResultsCount != players {
		t.Fatalf("match %s: expected %d game result rows, got %d", matchID, players, gameResultsCount)
	}

	commissionCents := invariantCents(platformFeeSum)
	if invariantCents(winSum)+commissionCents != grossPotCents {
		t.Fatalf(
			"match %s: win sum + commission mismatch got=%d+%d want=%d",
			matchID,
			invariantCents(winSum),
			commissionCents,
			grossPotCents,
		)
	}
	if invariantCents(gameResultsGross)+commissionCents != grossPotCents {
		t.Fatalf(
			"match %s: game_results gross + commission mismatch got=%d+%d want=%d",
			matchID,
			invariantCents(gameResultsGross),
			commissionCents,
			grossPotCents,
		)
	}
	if invariantCents(gameResultsNet) != -commissionCents {
		t.Fatalf(
			"match %s: game_results net mismatch got=%d want=%d",
			matchID,
			invariantCents(gameResultsNet),
			-commissionCents,
		)
	}

	return commissionCents
}

func sumBalancesCents(t *testing.T, ctx context.Context, walletSvc *wallet.Service, playerIDs []string) int64 {
	t.Helper()
	var total int64
	for _, playerID := range playerIDs {
		balance, err := walletSvc.Balance(ctx, playerID)
		if err != nil {
			t.Fatalf("balance for %s: %v", playerID, err)
		}
		total += invariantCents(balance)
	}
	return total
}

func invariantCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}
