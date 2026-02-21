package users

import (
	"context"
	"time"

	"durakonline/backend/pkg/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           string    `json:"id"`
	TelegramID   int64     `json:"telegram_id"`
	Username     string    `json:"username"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	PhotoURL     string    `json:"photo_url"`
	DisplayName  string    `json:"display_name"`
	Currency     string    `json:"currency"`
	ReferralCode string    `json:"referral_code"`
	InvitedBy    *string   `json:"invited_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetOrCreateByTelegram(
	ctx context.Context,
	telegramID int64,
	username string,
	firstName string,
	lastName string,
	photoURL string,
) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	ref := uuid.NewString()[:8]
	query := `
INSERT INTO users (telegram_id, username, first_name, last_name, photo_url, display_name, referral_code)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (telegram_id)
DO UPDATE SET
    username = EXCLUDED.username,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    photo_url = EXCLUDED.photo_url,
    display_name = CASE
        WHEN users.display_name = '' THEN EXCLUDED.display_name
        ELSE users.display_name
    END,
    updated_at = NOW()
RETURNING id, telegram_id, username, first_name, last_name, photo_url, display_name, currency, referral_code, invited_by, created_at, updated_at`

	displayName := firstName
	if username != "" {
		displayName = "@" + username
	}
	if displayName == "" {
		displayName = "Игрок"
	}

	var user User
	start := time.Now()
	err := r.db.QueryRow(ctx, query, telegramID, username, firstName, lastName, photoURL, displayName, ref).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.PhotoURL,
		&user.DisplayName,
		&user.Currency,
		&user.ReferralCode,
		&user.InvitedBy,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	metrics.ObserveDBQuery("upsert_user_by_telegram", start)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (User, bool) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT id, telegram_id, username, first_name, last_name, photo_url, display_name, currency, referral_code, invited_by, created_at, updated_at FROM users WHERE id=$1`
	var user User
	start := time.Now()
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.PhotoURL,
		&user.DisplayName,
		&user.Currency,
		&user.ReferralCode,
		&user.InvitedBy,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	metrics.ObserveDBQuery("select_user_by_id", start)
	if err != nil {
		return User{}, false
	}
	return user, true
}

func (r *Repository) UpdateSettings(ctx context.Context, id, displayName, currency string) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `
UPDATE users
SET
    display_name = COALESCE(NULLIF($2, ''), display_name),
    currency = COALESCE(NULLIF($3, ''), currency),
    updated_at = NOW()
WHERE id = $1
RETURNING id, telegram_id, username, first_name, last_name, photo_url, display_name, currency, referral_code, invited_by, created_at, updated_at`

	var user User
	start := time.Now()
	err := r.db.QueryRow(ctx, query, id, displayName, currency).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.PhotoURL,
		&user.DisplayName,
		&user.Currency,
		&user.ReferralCode,
		&user.InvitedBy,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	metrics.ObserveDBQuery("update_user_settings", start)
	if err != nil {
		return User{}, err
	}
	return user, nil
}
