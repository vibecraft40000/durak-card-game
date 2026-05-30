package wallet

import (
	"context"
	"errors"
	"math"
	"time"

	"durakonline/backend/internal/transactions"
	"durakonline/backend/pkg/metrics"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInsufficientBalance = errors.New("insufficient balance")

type Service struct {
	txRepo *transactions.Repository
	db     *pgxpool.Pool
}

func NewService(db *pgxpool.Pool, txRepo *transactions.Repository) *Service {
	return &Service{db: db, txRepo: txRepo}
}

func (s *Service) Balance(ctx context.Context, userID string) (float64, error) {
	return s.txRepo.Balance(ctx, userID)
}

func (s *Service) HoldBet(ctx context.Context, userID, matchID string, stake float64) error {
	for attempt := 0; attempt < 3; attempt++ {
		err := s.holdBetOnce(ctx, userID, matchID, stake)
		if !isRetryableTxErr(err) {
			return err
		}
		time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
	}
	return errors.New("hold bet failed after retries")
}

// RollbackHoldBet compensates a previously created bet_hold when match start fails.
func (s *Service) RollbackHoldBet(ctx context.Context, userID, matchID string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var heldNet float64
	start := time.Now()
	err = tx.QueryRow(ctx, `
SELECT COALESCE(SUM(amount), 0)
FROM transactions
WHERE user_id = $1
  AND match_id = $2
  AND status = 'confirmed'
  AND type IN ('bet_hold', 'bet_hold_release')`,
		userID, matchID,
	).Scan(&heldNet)
	metrics.ObserveDBQuery("tx_select_bet_hold_net", start)
	if err != nil {
		return err
	}
	if heldNet >= 0 {
		return tx.Commit(ctx)
	}

	releaseAmount := round2(-heldNet)
	start = time.Now()
	_, err = tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES (gen_random_uuid(), $1, 'bet_hold_release', $2, 'confirmed', $3, NOW())
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING`,
		userID, releaseAmount, matchID,
	)
	metrics.ObserveDBQuery("tx_insert_bet_hold_release", start)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) holdBetOnce(ctx context.Context, userID, matchID string, stake float64) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var balance float64
	qStart := time.Now()
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(amount),0) FROM transactions WHERE user_id=$1 AND status='confirmed'`, userID).Scan(&balance); err != nil {
		metrics.ObserveDBQuery("tx_select_balance", qStart)
		return err
	}
	metrics.ObserveDBQuery("tx_select_balance", qStart)
	if balance < stake {
		return ErrInsufficientBalance
	}

	qStart = time.Now()
	tag, err := tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES (gen_random_uuid(), $1, 'bet_hold', $2, 'confirmed', $3, NOW())
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING`, userID, -stake, matchID)
	metrics.ObserveDBQuery("tx_insert_bet_hold", qStart)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}

	return tx.Commit(ctx)
}

func (s *Service) SettleWin(ctx context.Context, winnerID, matchID string, pot float64, commissionBps int) error {
	for attempt := 0; attempt < 3; attempt++ {
		err := s.settleOnce(ctx, winnerID, matchID, pot, commissionBps)
		if !isRetryableTxErr(err) {
			return err
		}
		time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
	}
	metrics.IncSettlement("error")
	return errors.New("settlement failed after retries")
}

// SettleWinInTx performs wallet settlement within an existing transaction. Caller must commit/rollback.
func (s *Service) SettleWinInTx(ctx context.Context, tx pgx.Tx, winnerID, matchID string, pot float64, commissionBps int) error {
	pot = round2(pot)
	commission := round2(pot * float64(commissionBps) / 10000.0)
	winAmount := round2(pot - commission)
	start := time.Now()
	tag, err := tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES (gen_random_uuid(), $1, 'win', $2, 'confirmed', $3, NOW())
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING`, winnerID, winAmount, matchID)
	metrics.ObserveDBQuery("tx_settle_insert_win", start)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		metrics.IncSettlement("duplicate")
		return nil
	}
	start = time.Now()
	_, err = tx.Exec(ctx, `
INSERT INTO platform_fees (id, match_id, gross_pot, commission_bps, commission_amount, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW())
ON CONFLICT (match_id) DO NOTHING`, matchID, pot, commissionBps, commission)
	metrics.ObserveDBQuery("tx_settle_insert_platform_fee", start)
	if err != nil {
		return err
	}
	metrics.IncSettlement("success")
	return nil
}

func (s *Service) settleOnce(ctx context.Context, winnerID, matchID string, pot float64, commissionBps int) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		metrics.IncSettlement("error")
		return err
	}
	defer tx.Rollback(ctx)

	pot = round2(pot)
	commission := round2(pot * float64(commissionBps) / 10000.0)
	winAmount := round2(pot - commission)

	start := time.Now()
	tag, err := tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES (gen_random_uuid(), $1, 'win', $2, 'confirmed', $3, NOW())
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING`, winnerID, winAmount, matchID)
	metrics.ObserveDBQuery("tx_settle_insert_win", start)
	if err != nil {
		metrics.IncSettlement("error")
		return err
	}
	if tag.RowsAffected() == 0 {
		metrics.IncSettlement("duplicate")
		return tx.Commit(ctx)
	}

	start = time.Now()
	_, err = tx.Exec(ctx, `
INSERT INTO platform_fees (id, match_id, gross_pot, commission_bps, commission_amount, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW())
ON CONFLICT (match_id) DO NOTHING`, matchID, pot, commissionBps, commission)
	metrics.ObserveDBQuery("tx_settle_insert_platform_fee", start)
	if err != nil {
		metrics.IncSettlement("error")
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		metrics.IncSettlement("error")
		return err
	}
	metrics.IncSettlement("success")
	return nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func isRetryableTxErr(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001" || pgErr.Code == "40P01"
	}
	return false
}
