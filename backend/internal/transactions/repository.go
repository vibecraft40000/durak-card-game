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
	TypeAdminAdjust Type = "admin_adjust"

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

// StatsForUser возвращает агрегированную статистику по пользователю:
// суммарные пополнения и суммарные выводы (оба значения >= 0).
func (r *Repository) StatsForUser(ctx context.Context, userID string) (totalDeposits, totalWithdrawals float64, err error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Сумма всех подтверждённых депозитов.
	qDeposit := `SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE user_id=$1 AND type='deposit' AND status='confirmed'`
	start := time.Now()
	if err = r.db.QueryRow(ctx, qDeposit, userID).Scan(&totalDeposits); err != nil {
		metrics.ObserveDBQuery("select_user_deposits_sum", start)
		return 0, 0, err
	}
	metrics.ObserveDBQuery("select_user_deposits_sum", start)

	// Сумма всех подтверждённых выводов (в таблице хранятся отрицательные суммы, разворачиваем знак).
	qWithdraw := `SELECT COALESCE(-SUM(amount), 0) FROM transactions WHERE user_id=$1 AND type='withdraw' AND status='confirmed'`
	start = time.Now()
	if err = r.db.QueryRow(ctx, qWithdraw, userID).Scan(&totalWithdrawals); err != nil {
		metrics.ObserveDBQuery("select_user_withdrawals_sum", start)
		return totalDeposits, 0, err
	}
	metrics.ObserveDBQuery("select_user_withdrawals_sum", start)
	return totalDeposits, totalWithdrawals, nil
}

// WithdrawalRecord is a withdraw transaction with user info (for admin list).
type WithdrawalRecord struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Amount      float64   `json:"amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListWithdrawals returns recent withdraw transactions with user display name (for admin).
func (r *Repository) ListWithdrawals(ctx context.Context, limit int) ([]WithdrawalRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	query := `
SELECT t.id, t.user_id, t.amount, t.status, t.created_at,
       COALESCE(u.username, ''), COALESCE(u.display_name, u.first_name || ' ' || u.last_name, '')
FROM transactions t
JOIN users u ON u.id = t.user_id
WHERE t.type = $1
ORDER BY t.created_at DESC
LIMIT $2`
	rows, err := r.db.Query(ctx, query, TypeWithdraw, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []WithdrawalRecord
	for rows.Next() {
		var w WithdrawalRecord
		err := rows.Scan(&w.ID, &w.UserID, &w.Amount, &w.Status, &w.CreatedAt, &w.Username, &w.DisplayName)
		if err != nil {
			return nil, err
		}
		w.Amount = -w.Amount // stored as negative
		list = append(list, w)
	}
	return list, rows.Err()
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

type AdminAuditLog struct {
	ID           string     `json:"id"`
	Actor        string     `json:"actor"`
	Action       string     `json:"action"`
	TargetUserID string     `json:"target_user_id"`
	Amount       *float64   `json:"amount,omitempty"`
	Reason       string     `json:"reason"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (r *Repository) AddAdminAudit(ctx context.Context, actor, action, targetUserID string, amount *float64, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	start := time.Now()
	_, err := r.db.Exec(ctx, `
INSERT INTO admin_audit_logs (id, actor, action, target_user_id, amount, reason, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW())`,
		actor, action, targetUserID, amount, reason,
	)
	metrics.ObserveDBQuery("insert_admin_audit_log", start)
	return err
}

type OperationLogItem struct {
	Kind        string    `json:"kind"`
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Action      string    `json:"action"`
	Amount      float64   `json:"amount"`
	Details     string    `json:"details"`
}

func (r *Repository) ListOperationLogs(ctx context.Context, limit int) ([]OperationLogItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	query := `
SELECT *
FROM (
    SELECT
        'transaction' AS kind,
        t.id::text AS id,
        t.created_at AS created_at,
        t.user_id::text AS user_id,
        COALESCE(u.username, '') AS username,
        COALESCE(u.display_name, '') AS display_name,
        t.type::text AS action,
        t.amount::float8 AS amount,
        t.status::text AS details
    FROM transactions t
    LEFT JOIN users u ON u.id = t.user_id

    UNION ALL

    SELECT
        'admin_action' AS kind,
        a.id::text AS id,
        a.created_at AS created_at,
        a.target_user_id::text AS user_id,
        COALESCE(u.username, '') AS username,
        COALESCE(u.display_name, '') AS display_name,
        a.action AS action,
        COALESCE(a.amount, 0)::float8 AS amount,
        COALESCE(a.reason, '') AS details
    FROM admin_audit_logs a
    LEFT JOIN users u ON u.id = a.target_user_id

    UNION ALL

    SELECT
        'game_result' AS kind,
        g.id::text AS id,
        g.created_at AS created_at,
        g.user_id::text AS user_id,
        COALESCE(u.username, '') AS username,
        COALESCE(u.display_name, '') AS display_name,
        'match_result' AS action,
        g.profit::float8 AS amount,
        ('match:' || g.match_id::text) AS details
    FROM game_results g
    LEFT JOIN users u ON u.id = g.user_id
) x
ORDER BY created_at DESC
LIMIT $1`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]OperationLogItem, 0, limit)
	for rows.Next() {
		var item OperationLogItem
		if err := rows.Scan(
			&item.Kind,
			&item.ID,
			&item.CreatedAt,
			&item.UserID,
			&item.Username,
			&item.DisplayName,
			&item.Action,
			&item.Amount,
			&item.Details,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
