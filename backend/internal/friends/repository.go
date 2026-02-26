package friends

import (
	"context"
	"time"

	"durakonline/backend/pkg/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusBlocked  = "blocked"
)

type Friend struct {
	ID        string
	UserID    string
	FriendID  string
	Status    string
	CreatedAt time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateRequest(ctx context.Context, userID, friendID string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	query := `INSERT INTO friends (user_id, friend_id, status) VALUES ($1, $2, $3)
ON CONFLICT (user_id, friend_id) DO NOTHING`
	start := time.Now()
	_, err := r.db.Exec(ctx, query, userID, friendID, StatusPending)
	metrics.ObserveDBQuery("friends_create_request", start)
	return err
}

func (r *Repository) Accept(ctx context.Context, userID, requestID string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// requestID is the friend_id of the requester (the one who sent the request)
	// We need: requester_id=requestID, acceptor_id=userID. Row has user_id=requester, friend_id=acceptor
	// So the row is: user_id=requestID, friend_id=userID, status=pending
	query := `UPDATE friends SET status = $3 WHERE user_id = $1 AND friend_id = $2 AND status = $4`
	start := time.Now()
	tag, err := r.db.Exec(ctx, query, requestID, userID, StatusAccepted, StatusPending)
	metrics.ObserveDBQuery("friends_accept", start)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRequestNotFound
	}
	// Create reverse row for symmetric friendship
	_, _ = r.db.Exec(ctx, `INSERT INTO friends (user_id, friend_id, status) VALUES ($1, $2, $3)
ON CONFLICT (user_id, friend_id) DO UPDATE SET status = $3`, userID, requestID, StatusAccepted)
	return nil
}

func (r *Repository) ListAccepted(ctx context.Context, userID string) ([]Friend, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	query := `SELECT id, user_id, friend_id, status, created_at FROM friends
WHERE status = $1 AND (user_id = $2 OR friend_id = $2)`
	start := time.Now()
	rows, err := r.db.Query(ctx, query, StatusAccepted, userID)
	metrics.ObserveDBQuery("friends_list", start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Friend
	for rows.Next() {
		var f Friend
		if err := rows.Scan(&f.ID, &f.UserID, &f.FriendID, &f.Status, &f.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

func (r *Repository) ListIncomingRequests(ctx context.Context, userID string) ([]Friend, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// Incoming: user_id=requester, friend_id=me, status=pending
	query := `SELECT id, user_id, friend_id, status, created_at FROM friends
WHERE friend_id = $1 AND status = $2`
	start := time.Now()
	rows, err := r.db.Query(ctx, query, userID, StatusPending)
	metrics.ObserveDBQuery("friends_list_requests", start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Friend
	for rows.Next() {
		var f Friend
		if err := rows.Scan(&f.ID, &f.UserID, &f.FriendID, &f.Status, &f.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

func (r *Repository) RemoveFriend(ctx context.Context, userID, friendID string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	query := `DELETE FROM friends
WHERE (user_id = $1 AND friend_id = $2)
   OR (user_id = $2 AND friend_id = $1)`
	start := time.Now()
	_, err := r.db.Exec(ctx, query, userID, friendID)
	metrics.ObserveDBQuery("friends_remove", start)
	return err
}
