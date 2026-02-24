package gameresults

import (
	"context"
	"fmt"
	"strings"
	"time"

	"durakonline/backend/pkg/metrics"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Entry struct {
	MatchID   string
	UserID    string
	Stake     float64
	Payout    float64
	Profit    float64
	CreatedAt time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(ctx context.Context, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	valueStrings := make([]string, 0, len(entries))
	valueArgs := make([]any, 0, len(entries)*5)
	for i, e := range entries {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)", i*5+1, i*5+2, i*5+3, i*5+4, i*5+5))
		valueArgs = append(valueArgs, e.MatchID, e.UserID, e.Stake, e.Payout, e.Profit)
	}
	query := `INSERT INTO game_results (match_id, user_id, stake, payout, profit) VALUES ` + strings.Join(valueStrings, ",")
	start := time.Now()
	_, err := r.db.Exec(ctx, query, valueArgs...)
	metrics.ObserveDBQuery("game_results_insert_batch", start)
	return err
}

// InsertWithTx inserts game_results within an existing transaction.
func (r *Repository) InsertWithTx(ctx context.Context, tx pgx.Tx, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	valueStrings := make([]string, 0, len(entries))
	valueArgs := make([]any, 0, len(entries)*5)
	for i, e := range entries {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)", i*5+1, i*5+2, i*5+3, i*5+4, i*5+5))
		valueArgs = append(valueArgs, e.MatchID, e.UserID, e.Stake, e.Payout, e.Profit)
	}
	query := `INSERT INTO game_results (match_id, user_id, stake, payout, profit) VALUES ` + strings.Join(valueStrings, ",")
	start := time.Now()
	_, err := tx.Exec(ctx, query, valueArgs...)
	metrics.ObserveDBQuery("game_results_insert_batch", start)
	return err
}
