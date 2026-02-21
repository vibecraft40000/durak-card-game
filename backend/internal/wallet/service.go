package wallet

import (
	"context"
	"errors"
	"time"

	"durakonline/backend/internal/transactions"
	"durakonline/backend/pkg/metrics"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5"
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

func (s *Service) settleOnce(ctx context.Context, winnerID, matchID string, pot float64, commissionBps int) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		metrics.IncSettlement("error")
		return err
	}
	defer tx.Rollback(ctx)

	commission := pot * float64(commissionBps) / 10000.0
	winAmount := pot - commission

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
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES (gen_random_uuid(), $1, 'commission', $2, 'confirmed', $3, NOW())
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING`, winnerID, -commission, matchID)
	metrics.ObserveDBQuery("tx_settle_insert_commission", start)
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
