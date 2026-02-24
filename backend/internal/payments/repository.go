package payments

import (
	"context"
	"time"

	"durakonline/backend/pkg/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, p *Payment) error {
	p.ID = uuid.NewString()
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	start := time.Now()
	_, err := r.db.Exec(ctx, `
INSERT INTO payments (id, user_id, external_id, wallet_order_id, amount_usd, currency_code, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		p.ID, p.UserID, p.ExternalID, nullStr(p.WalletOrderID), p.AmountUSD, p.CurrencyCode, string(p.Status), p.CreatedAt, p.UpdatedAt)
	metrics.ObserveDBQuery("insert_payment", start)
	return err
}

func (r *Repository) GetByExternalID(ctx context.Context, externalID string) (*Payment, error) {
	start := time.Now()
	var p Payment
	var paidAt *time.Time
	var paidAmount *float64
	var rawWebhook []byte
	err := r.db.QueryRow(ctx, `
SELECT id, user_id, external_id, wallet_order_id, amount_usd, currency_code, paid_amount, paid_currency, status, created_at, paid_at, updated_at, COALESCE(raw_webhook, '{}'::jsonb)
FROM payments WHERE external_id = $1`, externalID).Scan(
		&p.ID, &p.UserID, &p.ExternalID, &p.WalletOrderID, &p.AmountUSD, &p.CurrencyCode,
		&paidAmount, &p.PaidCurrency, &p.Status, &p.CreatedAt, &paidAt, &p.UpdatedAt, &rawWebhook)
	metrics.ObserveDBQuery("select_payment_by_external_id", start)
	if err != nil {
		return nil, err
	}
	p.PaidAt = paidAt
	p.PaidAmount = paidAmount
	p.RawWebhook = rawWebhook
	return &p, nil
}

func (r *Repository) UpdatePaid(ctx context.Context, id string, paidAmount float64, paidCurrency string, rawWebhook []byte) error {
	start := time.Now()
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
UPDATE payments SET status = 'paid', paid_at = $2, paid_amount = $3, paid_currency = $4, raw_webhook = $5, updated_at = $6 WHERE id = $1`,
		id, now, paidAmount, paidCurrency, rawWebhook, now)
	metrics.ObserveDBQuery("update_payment_paid", start)
	return err
}

func (r *Repository) SetWalletOrderID(ctx context.Context, id, walletOrderID string) error {
	start := time.Now()
	_, err := r.db.Exec(ctx, `UPDATE payments SET wallet_order_id = $2, updated_at = NOW() WHERE id = $1`, id, walletOrderID)
	metrics.ObserveDBQuery("update_payment_wallet_order_id", start)
	return err
}

// InsertCryptoPayPaid creates a payments record for a CryptoPay deposit (audit trail).
// externalID should be "cryptopay:" + invoiceID to avoid collision with WalletPay.
func (r *Repository) InsertCryptoPayPaid(ctx context.Context, userID, invoiceID string, amountUSD float64, rawWebhook []byte) error {
	externalID := "cryptopay:" + invoiceID
	p := &Payment{
		UserID:     userID,
		ExternalID: externalID,
		AmountUSD:  amountUSD,
		CurrencyCode: "USD",
		Status:     StatusPaid,
	}
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	if rawWebhook == nil {
		rawWebhook = []byte("{}")
	}
	start := time.Now()
	_, err := r.db.Exec(ctx, `
INSERT INTO payments (id, user_id, external_id, amount_usd, currency_code, status, paid_at, created_at, updated_at, raw_webhook)
VALUES (gen_random_uuid(), $1, $2, $3, $4, 'paid', $5, $5, $5, $6)
ON CONFLICT (external_id) DO NOTHING`, userID, externalID, amountUSD, "USD", p.CreatedAt, rawWebhook)
	metrics.ObserveDBQuery("insert_cryptopay_payment", start)
	return err
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
