package users

import (
	"context"
	"strings"
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
	Language     string    `json:"language"`
	ReferralCode string    `json:"referral_code"`
	InvitedBy    *string   `json:"invited_by,omitempty"`
	IsBanned     bool      `json:"is_banned"`
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
    display_name = EXCLUDED.display_name,
    updated_at = NOW()
RETURNING id, telegram_id, username, first_name, last_name, photo_url, display_name, currency, COALESCE(language, 'ru'), referral_code, invited_by, created_at, updated_at`

	// Ник = Имя Фамилия (не @username), обновляется при каждом входе
	displayName := strings.TrimSpace(firstName + " " + lastName)
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
		&user.Language,
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

func (r *Repository) ResolveID(ctx context.Context, idOrUsername string) (string, bool) {
	// If it looks like UUID, try GetByID
	if len(idOrUsername) == 36 && idOrUsername[8] == '-' {
		u, ok := r.GetByID(ctx, idOrUsername)
		if ok {
			return u.ID, true
		}
		return "", false
	}
	// Try by username (with or without @)
	uname := strings.TrimPrefix(idOrUsername, "@")
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var id string
	err := r.db.QueryRow(ctx, `SELECT id FROM users WHERE username = $1 OR username = $2`, uname, idOrUsername).Scan(&id)
	if err != nil {
		return "", false
	}
	return id, true
}

func (r *Repository) GetByID(ctx context.Context, id string) (User, bool) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT id, telegram_id, username, first_name, last_name, photo_url, display_name, currency, COALESCE(language, 'ru'), referral_code, invited_by, COALESCE(is_banned, false), created_at, updated_at FROM users WHERE id=$1`
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
		&user.Language,
		&user.ReferralCode,
		&user.InvitedBy,
		&user.IsBanned,
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
RETURNING id, telegram_id, username, first_name, last_name, photo_url, display_name, currency, COALESCE(language, 'ru'), referral_code, invited_by, created_at, updated_at`

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
		&user.Language,
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

func (r *Repository) UpdateLanguage(ctx context.Context, id, language string) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	valid := map[string]bool{"ru": true, "uk": true, "en": true}
	if !valid[language] {
		language = "ru"
	}
	query := `UPDATE users SET language = $2, updated_at = NOW() WHERE id = $1
RETURNING id, telegram_id, username, first_name, last_name, photo_url, display_name, currency, language, referral_code, invited_by, created_at, updated_at`
	var user User
	start := time.Now()
	err := r.db.QueryRow(ctx, query, id, language).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.PhotoURL, &user.DisplayName, &user.Currency, &user.Language,
		&user.ReferralCode, &user.InvitedBy, &user.CreatedAt, &user.UpdatedAt,
	)
	metrics.ObserveDBQuery("update_user_language", start)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (r *Repository) UpdatePhotoURL(ctx context.Context, id, photoURL string) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	query := `UPDATE users SET photo_url = $2, updated_at = NOW() WHERE id = $1
RETURNING id, telegram_id, username, first_name, last_name, photo_url, display_name, currency, COALESCE(language, 'ru'), referral_code, invited_by, created_at, updated_at`
	var user User
	start := time.Now()
	err := r.db.QueryRow(ctx, query, id, photoURL).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.PhotoURL, &user.DisplayName, &user.Currency, &user.Language,
		&user.ReferralCode, &user.InvitedBy, &user.CreatedAt, &user.UpdatedAt,
	)
	metrics.ObserveDBQuery("update_user_photo_url", start)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// Count returns total number of users (for admin stats).
func (r *Repository) Count(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var n int
	start := time.Now()
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	metrics.ObserveDBQuery("users_count", start)
	return n, err
}

// ListPaginated returns users for admin panel (offset, limit) and total count.
func (r *Repository) ListPaginated(ctx context.Context, offset, limit int) ([]User, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	start := time.Now()
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)
	metrics.ObserveDBQuery("users_count", start)
	if err != nil {
		return nil, 0, err
	}
	query := `SELECT id, telegram_id, username, first_name, last_name, photo_url, display_name, currency, COALESCE(language, 'ru'), referral_code, invited_by, COALESCE(is_banned, false), created_at, updated_at FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []User
	for rows.Next() {
		var u User
		err := rows.Scan(&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.LastName, &u.PhotoURL, &u.DisplayName, &u.Currency, &u.Language, &u.ReferralCode, &u.InvitedBy, &u.IsBanned, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, u)
	}
	return list, total, rows.Err()
}

func (r *Repository) SetBanned(ctx context.Context, id string, banned bool) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	start := time.Now()
	_, err := r.db.Exec(ctx, `UPDATE users SET is_banned=$2, updated_at=NOW() WHERE id=$1`, id, banned)
	metrics.ObserveDBQuery("set_user_banned", start)
	return err
}
