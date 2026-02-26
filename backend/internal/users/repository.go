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
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "ua" {
		language = "uk"
	}
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

func (r *Repository) BindInviterByReferralCode(ctx context.Context, userID, referralCode string) error {
	referralCode = strings.TrimSpace(referralCode)
	if referralCode == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	start := time.Now()
	_, err := r.db.Exec(ctx, `
UPDATE users target
SET invited_by = inviter.id,
    updated_at = NOW()
FROM users inviter
WHERE target.id = $1
  AND target.invited_by IS NULL
  AND lower(inviter.referral_code) = lower($2)
  AND inviter.id <> target.id`, userID, referralCode)
	metrics.ObserveDBQuery("bind_inviter_by_referral_code", start)
	return err
}

type ReferralInvite struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	JoinedAt    time.Time `json:"joined_at"`
	GamesPlayed int64     `json:"games_played"`
	DepositsUSD float64   `json:"deposits_usd"`
}

type ReferralStats struct {
	TotalInvited  int64            `json:"total_invited"`
	ActiveInvited int64            `json:"active_invited"`
	TotalGames    int64            `json:"total_games"`
	TotalDeposits float64          `json:"total_deposits_usd"`
	RecentInvites []ReferralInvite `json:"recent_invites"`
}

func (r *Repository) GetReferralStats(ctx context.Context, userID string, limit int) (ReferralStats, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	var stats ReferralStats
	summaryQuery := `
SELECT
	COUNT(*)::bigint AS total_invited,
	COUNT(*) FILTER (WHERE EXISTS (
		SELECT 1 FROM game_results g WHERE g.user_id = u.id
	))::bigint AS active_invited,
	COALESCE(SUM((
		SELECT COUNT(*)::bigint FROM game_results g WHERE g.user_id = u.id
	)), 0)::bigint AS total_games,
	COALESCE(SUM((
		SELECT COALESCE(SUM(t.amount), 0)::float8
		FROM transactions t
		WHERE t.user_id = u.id
		  AND t.type = 'deposit'
		  AND t.status = 'confirmed'
	)), 0)::float8 AS total_deposits_usd
FROM users u
WHERE u.invited_by = $1`
	start := time.Now()
	if err := r.db.QueryRow(ctx, summaryQuery, userID).Scan(
		&stats.TotalInvited,
		&stats.ActiveInvited,
		&stats.TotalGames,
		&stats.TotalDeposits,
	); err != nil {
		metrics.ObserveDBQuery("referral_stats_summary", start)
		return ReferralStats{}, err
	}
	metrics.ObserveDBQuery("referral_stats_summary", start)

	listQuery := `
SELECT
	u.id::text,
	COALESCE(u.username, '') AS username,
	COALESCE(u.display_name, '') AS display_name,
	u.created_at,
	COALESCE((
		SELECT COUNT(*)::bigint FROM game_results g WHERE g.user_id = u.id
	), 0)::bigint AS games_played,
	COALESCE((
		SELECT SUM(t.amount)::float8
		FROM transactions t
		WHERE t.user_id = u.id
		  AND t.type = 'deposit'
		  AND t.status = 'confirmed'
	), 0)::float8 AS deposits_usd
FROM users u
WHERE u.invited_by = $1
ORDER BY u.created_at DESC
LIMIT $2`
	start = time.Now()
	rows, err := r.db.Query(ctx, listQuery, userID, limit)
	metrics.ObserveDBQuery("referral_stats_list", start)
	if err != nil {
		return ReferralStats{}, err
	}
	defer rows.Close()

	stats.RecentInvites = make([]ReferralInvite, 0, limit)
	for rows.Next() {
		var item ReferralInvite
		if err := rows.Scan(
			&item.UserID,
			&item.Username,
			&item.DisplayName,
			&item.JoinedAt,
			&item.GamesPlayed,
			&item.DepositsUSD,
		); err != nil {
			return ReferralStats{}, err
		}
		stats.RecentInvites = append(stats.RecentInvites, item)
	}
	if err := rows.Err(); err != nil {
		return ReferralStats{}, err
	}
	return stats, nil
}
