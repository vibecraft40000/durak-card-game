package transactions

import (
	"context"
	"time"

	"durakonline/backend/pkg/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Type string
type Status string

const (
	TypeDeposit    Type = "deposit"
	TypeWithdraw   Type = "withdraw"
	TypeBetHold    Type = "bet_hold"
	TypeWin        Type = "win"
	TypeCommission Type = "commission"

	StatusPending   Status = "pending"
	StatusConfirmed Status = "confirmed"
	StatusFailed    Status = "failed"
)

type Transaction struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      Type      `json:"type"`
	Amount    float64   `json:"amount"`
	Status    Status    `json:"status"`
	MatchID   string    `json:"match_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Add(ctx context.Context, tx Transaction) (Transaction, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx.ID = uuid.NewString()
	tx.CreatedAt = time.Now().UTC()
	query := `
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)`
	start := time.Now()
	_, err := r.db.Exec(ctx, query, tx.ID, tx.UserID, tx.Type, tx.Amount, tx.Status, nullIfEmpty(tx.MatchID), tx.CreatedAt)
	metrics.ObserveDBQuery("insert_transaction", start)
	if err != nil {
		return Transaction{}, err
	}
	return tx, nil
}

func (r *Repository) AddIdempotent(ctx context.Context, tx Transaction) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx.ID = uuid.NewString()
	tx.CreatedAt = time.Now().UTC()
	query := `
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING`
	start := time.Now()
	tag, err := r.db.Exec(ctx, query, tx.ID, tx.UserID, tx.Type, tx.Amount, tx.Status, nullIfEmpty(tx.MatchID), tx.CreatedAt)
	metrics.ObserveDBQuery("insert_transaction_idempotent", start)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repository) Balance(ctx context.Context, userID string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	query := `SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE user_id=$1 AND status='confirmed'`
	var total float64
	start := time.Now()
	if err := r.db.QueryRow(ctx, query, userID).Scan(&total); err != nil {
		metrics.ObserveDBQuery("select_balance", start)
		return 0, err
	}
	metrics.ObserveDBQuery("select_balance", start)
	return total, nil
}

func (r *Repository) SeedDeposit(userID string, amount float64) {
	_, _ = r.Add(context.Background(), Transaction{
		UserID: userID,
		Type:   TypeDeposit,
		Amount: amount,
		Status: StatusConfirmed,
	})
}

func nullIfEmpty(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
