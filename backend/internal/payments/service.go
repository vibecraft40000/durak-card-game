package payments

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"

	"durakonline/backend/internal/transactions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAmountMismatch = errors.New("payment amount mismatch")

type Service struct {
	db      *pgxpool.Pool
	repo    *Repository
	client  *Client
	txRepo  *transactions.Repository
}

func NewService(db *pgxpool.Pool, repo *Repository, client *Client, txRepo *transactions.Repository) *Service {
	return &Service{db: db, repo: repo, client: client, txRepo: txRepo}
}

// CreateInvoice creates a pending payment, calls WalletPay API, returns directPayLink.
func (s *Service) CreateInvoice(ctx context.Context, userID string, amountUSD float64, telegramUserID int64) (payLink, externalID string, err error) {
	if amountUSD < 1 || amountUSD > 1000 {
		return "", "", errors.New("amount must be 1-1000 USD")
	}

	externalID = uuid.NewString()
	desc := "Durak Online — пополнение баланса"
	amountStr := strconv.FormatFloat(amountUSD, 'f', 2, 64)

	req := CreateOrderReq{
		Amount:                 MoneyAmount{CurrencyCode: "USD", Amount: amountStr},
		ExternalID:             externalID,
		TimeoutSeconds:         3600,
		Description:            desc,
		CustomerTelegramUserID: telegramUserID,
		AutoConversionCurrency: "USDT",
	}

	directPayLink, orderID, err := s.client.CreateOrder(ctx, req)
	if err != nil {
		return "", "", err
	}

	p := &Payment{
		UserID:        userID,
		ExternalID:    externalID,
		AmountUSD:     amountUSD,
		CurrencyCode:  "USD",
		Status:        StatusPending,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return "", "", err
	}
	_ = s.repo.SetWalletOrderID(ctx, p.ID, strconv.FormatInt(orderID, 10))

	return directPayLink, externalID, nil
}

// HandleOrderPaid processes ORDER_PAID webhook idempotently.
func (s *Service) HandleOrderPaid(ctx context.Context, payload WebhookPayload, rawWebhook []byte) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var p Payment
	var paidAt *time.Time
	err = tx.QueryRow(ctx, `
SELECT id, user_id, external_id, amount_usd, status, paid_at
FROM payments WHERE external_id = $1`, payload.ExternalID).Scan(
		&p.ID, &p.UserID, &p.ExternalID, &p.AmountUSD, &p.Status, &paidAt)
	if err != nil {
		return err
	}

	if p.Status == StatusPaid {
		return nil
	}

	webhookAmount, err := parseOrderAmountUSD(payload.OrderAmount)
	if err != nil {
		return err
	}
	if math.Abs(webhookAmount-p.AmountUSD) > 0.001 {
		return ErrAmountMismatch
	}

	paidAmt := webhookAmount
	paidCur := payload.OrderAmount.CurrencyCode
	if payload.SelectedPaymentOption != nil {
		overrideAmt, err := parseOrderAmountUSD(payload.SelectedPaymentOption.Amount)
		if err != nil {
			return err
		}
		paidAmt = overrideAmt
		paidCur = payload.SelectedPaymentOption.Amount.CurrencyCode
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
UPDATE payments SET status = 'paid', paid_at = $2, paid_amount = $3, paid_currency = $4, raw_webhook = $5, updated_at = $6 WHERE id = $1`,
		p.ID, now, paidAmt, paidCur, rawWebhook, now)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO transactions (id, user_id, type, amount, status, created_at)
VALUES (gen_random_uuid(), $1, 'deposit', $2, 'confirmed', $3)`,
		p.UserID, p.AmountUSD, now)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func parseOrderAmountUSD(m MoneyAmount) (float64, error) {
	if m.CurrencyCode == "USD" {
		return strconv.ParseFloat(m.Amount, 64)
	}
	return 0, errors.New("unsupported currency for amount check")
}

// ParseWebhookEvents parses WalletPay webhook body
func ParseWebhookEvents(data []byte) ([]WebhookEvent, error) {
	var events []WebhookEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}
