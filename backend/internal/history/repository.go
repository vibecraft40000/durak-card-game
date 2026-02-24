package history

import (
	"context"
	"fmt"
	"time"

	"durakonline/backend/pkg/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Item struct {
	MatchID   string    `json:"matchId"`
	Stake     float64   `json:"stake"`
	Payout    float64   `json:"payout"`
	Profit    float64   `json:"profit"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"createdAt"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByUser(ctx context.Context, userID string, limit, offset int, from, to *time.Time) ([]Item, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	where := "WHERE user_id = $1"
	args := []any{userID}
	n := 2
	if from != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", n)
		args = append(args, *from)
		n++
	}
	if to != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", n)
		args = append(args, *to)
		n++
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM game_results " + where
	start := time.Now()
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	metrics.ObserveDBQuery("history_count", start)
	if err != nil {
		return nil, 0, err
	}

	// Fetch items with pagination
	listArgs := append(args, limit, offset)
	listQuery := `SELECT match_id, stake, payout, profit, created_at
		FROM game_results ` + where + fmt.Sprintf(`
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, n, n+1)
	start = time.Now()
	rows, err := r.db.Query(ctx, listQuery, listArgs...)
	metrics.ObserveDBQuery("history_list", start)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.MatchID, &it.Stake, &it.Payout, &it.Profit, &it.CreatedAt); err != nil {
			return nil, 0, err
		}
		if it.Profit > 0 {
			it.Result = "win"
		} else {
			it.Result = "loss"
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

type CalendarDay struct {
	Date   string  `json:"date"`
	Games  int     `json:"games"`
	Profit float64 `json:"profit"`
}

func (r *Repository) CalendarByMonth(ctx context.Context, userID string, year int, month int) ([]CalendarDay, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	query := `SELECT created_at::date AS d, COUNT(*) AS cnt, COALESCE(SUM(profit), 0) AS p
FROM game_results
WHERE user_id = $1 AND EXTRACT(YEAR FROM created_at) = $2 AND EXTRACT(MONTH FROM created_at) = $3
GROUP BY created_at::date
ORDER BY d`
	start := time.Now()
	rows, err := r.db.Query(ctx, query, userID, year, month)
	metrics.ObserveDBQuery("history_calendar", start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var days []CalendarDay
	for rows.Next() {
		var d time.Time
		var cnt int64
		var p float64
		if err := rows.Scan(&d, &cnt, &p); err != nil {
			return nil, err
		}
		days = append(days, CalendarDay{
			Date:   d.Format("2006-01-02"),
			Games:  int(cnt),
			Profit: p,
		})
	}
	return days, rows.Err()
}
