package cryptopay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"durakonline/backend/internal/money"
	"durakonline/backend/internal/payments"
	"durakonline/backend/internal/transactions"
	"durakonline/backend/internal/users"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/pkg/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const paidInvoiceTTL = 30 * 24 * time.Hour

type Handler struct {
	client             *Client
	txRepo             *transactions.Repository
	paymentsRepo       *payments.Repository
	redis              *redis.Client
	token              string
	webappURL          string
	log                *zap.Logger
	notifyBotToken     string
	notifyChatIDs      []int64
	converter          *money.Converter
	withdrawCardFeeBps int
	withdrawCryptoBps  int
}

func NewHandler(token string, testnet bool, txRepo *transactions.Repository, r *redis.Client, webappURL string, log *zap.Logger) *Handler {
	useTestnet := testnet || strings.HasPrefix(token, "test")
	client := NewClient(token, useTestnet)
	converter, _ := money.NewConverter("")
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{
		client:             client,
		txRepo:             txRepo,
		paymentsRepo:       nil,
		redis:              r,
		token:              token,
		webappURL:          webappURL,
		log:                log,
		converter:          converter,
		withdrawCardFeeBps: 200,
		withdrawCryptoBps:  0,
	}
}

// WithPaymentsRepo enables creating payments records for CryptoPay deposits (audit trail).
func (h *Handler) WithPaymentsRepo(repo *payments.Repository) *Handler {
	h.paymentsRepo = repo
	return h
}

// WithNotifyOnWithdraw sends a Telegram message to admin(s) when a withdraw succeeds.
func (h *Handler) WithNotifyOnWithdraw(botToken string, chatIDs []int64) *Handler {
	h.notifyBotToken = botToken
	h.notifyChatIDs = chatIDs
	return h
}

func (h *Handler) WithMoneyConverter(converter *money.Converter) *Handler {
	if converter != nil {
		h.converter = converter
	}
	return h
}

func (h *Handler) WithWithdrawFees(cardFeeBps, cryptoFeeBps int) *Handler {
	if cardFeeBps < 0 {
		cardFeeBps = 0
	}
	if cryptoFeeBps < 0 {
		cryptoFeeBps = 0
	}
	h.withdrawCardFeeBps = cardFeeBps
	h.withdrawCryptoBps = cryptoFeeBps
	return h
}

const withdrawLockTTL = 30 * time.Second

const (
	withdrawMethodCrypto = "crypto"
	withdrawMethodCard   = "card"
)

type withdrawNotification struct {
	Method         string
	SourceAmount   float64
	SourceCurrency string
	SourceRateUSD  float64
	DebitUSD       float64
	FeeBps         int
	FeeUSD         float64
	PayoutUSD      float64
	BalanceBefore  float64
}

func (h *Handler) CreateWithdraw(walletService *wallet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, ok := middleware.UserFromContext(ctx)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if user.TelegramID == 0 {
			http.Error(w, "telegram account required for withdraw", http.StatusBadRequest)
			return
		}

		var req struct {
			Amount   float64 `json:"amount"`
			Currency string  `json:"currency"`
			Method   string  `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		method, err := normalizeWithdrawMethod(req.Method)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		sourceCurrency := money.NormalizeCurrency(req.Currency)
		amountUSD, sourceRateUSD, err := h.toUSD(req.Amount, sourceCurrency)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if amountUSD < 5 || amountUSD > 1000 {
			http.Error(w, "amount must be 5-1000 USD equivalent", http.StatusBadRequest)
			return
		}

		feeBps := h.withdrawFeeBps(method)
		feeUSD := money.Round2(amountUSD * float64(feeBps) / 10000.0)
		payoutUSD := money.Round2(amountUSD - feeUSD)
		if payoutUSD <= 0 {
			http.Error(w, "withdraw amount too small after fee", http.StatusBadRequest)
			return
		}

		balance, err := walletService.Balance(ctx, user.ID)
		if err != nil {
			h.log.Error("withdraw: balance check failed", zap.Error(err))
			http.Error(w, "failed to check balance", http.StatusInternalServerError)
			return
		}
		if balance < amountUSD {
			http.Error(w, "insufficient balance", http.StatusBadRequest)
			return
		}

		lockKey := "withdraw:lock:" + user.ID
		if h.redis != nil {
			okLock, lockErr := h.redis.SetNX(ctx, lockKey, "1", withdrawLockTTL).Result()
			if lockErr != nil || !okLock {
				http.Error(w, "withdraw in progress or too many attempts", http.StatusTooManyRequests)
				return
			}
			defer h.redis.Del(ctx, lockKey)
		}

		_, err = h.txRepo.Add(ctx, transactions.Transaction{
			UserID: user.ID,
			Type:   transactions.TypeWithdraw,
			Amount: -amountUSD,
			Status: transactions.StatusConfirmed,
		})
		if err != nil {
			h.log.Error("withdraw: debit failed", zap.Error(err))
			http.Error(w, "failed to process withdraw", http.StatusInternalServerError)
			return
		}

		h.notifyWithdraw(ctx, &user, withdrawNotification{
			Method:         method,
			SourceAmount:   req.Amount,
			SourceCurrency: sourceCurrency,
			SourceRateUSD:  sourceRateUSD,
			DebitUSD:       amountUSD,
			FeeBps:         feeBps,
			FeeUSD:         feeUSD,
			PayoutUSD:      payoutUSD,
			BalanceBefore:  balance,
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":          "ok",
			"transferId":      0,
			"asset":           "USD",
			"amount":          amountUSD,
			"method":          method,
			"currency":        sourceCurrency,
			"sourceAmount":    req.Amount,
			"sourceUsdRate":   sourceRateUSD,
			"debitAmountUsd":  amountUSD,
			"feeBps":          feeBps,
			"feeAmountUsd":    feeUSD,
			"payoutAmountUsd": payoutUSD,
		})
	}
}

func (h *Handler) notifyWithdraw(ctx context.Context, user *users.User, data withdrawNotification) {
	if h.notifyBotToken == "" || len(h.notifyChatIDs) == 0 {
		return
	}
	displayName := user.DisplayName
	if displayName == "" {
		displayName = user.Username
	}
	if displayName == "" {
		displayName = fmt.Sprintf("ID %s", user.ID[:8])
	}
	usernameLine := ""
	if strings.TrimSpace(user.Username) != "" {
		usernameLine = "@" + user.Username + "\n"
	}
	var deposits, withdrawals float64
	if h.txRepo != nil {
		if d, wtot, err := h.txRepo.StatsForUser(ctx, user.ID); err == nil {
			deposits, withdrawals = d, wtot
		} else {
			h.log.Warn("notify withdraw: stats failed", zap.String("user_id", user.ID), zap.Error(err))
		}
	}
	balanceAfter := data.BalanceBefore - data.DebitUSD
	methodTitle := strings.ToUpper(data.Method)

	text := fmt.Sprintf(
		"<b>Withdraw request</b>\n\n"+
			"User: %s\n%sTelegram ID: <code>%d</code>\n"+
			"<a href=\"tg://user?id=%d\">Open Telegram profile</a>\n\n"+
			"Method: <b>%s</b>\n"+
			"Requested: <b>%.2f %s</b>\n"+
			"FX rate: <b>1 %s = %.6f USD</b>\n"+
			"Debit from balance: <b>%.2f USD</b>\n"+
			"Fee: <b>%d bps (%.2f USD)</b>\n"+
			"Payout to user: <b>%.2f USD</b>\n"+
			"Balance before: <b>%.2f USD</b>\n"+
			"Balance after: <b>%.2f USD</b>\n\n"+
			"Deposits total: <b>%.2f USD</b>\n"+
			"Withdrawals total: <b>%.2f USD</b>",
		displayName, usernameLine, user.TelegramID, user.TelegramID,
		methodTitle, data.SourceAmount, data.SourceCurrency,
		data.SourceCurrency, data.SourceRateUSD,
		data.DebitUSD, data.FeeBps, data.FeeUSD, data.PayoutUSD,
		data.BalanceBefore, balanceAfter, deposits, withdrawals,
	)

	body := map[string]any{"chat_id": 0, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true}
	for _, chatID := range h.notifyChatIDs {
		body["chat_id"] = chatID
		raw, _ := json.Marshal(body)
		resp, err := http.Post("https://api.telegram.org/bot"+h.notifyBotToken+"/sendMessage", "application/json", bytes.NewReader(raw))
		if err != nil {
			h.log.Warn("notify withdraw: send failed", zap.Int64("chat_id", chatID), zap.Error(err))
			continue
		}
		resp.Body.Close()
	}
}

func (h *Handler) CreateDepositInvoice(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	sourceCurrency := money.NormalizeCurrency(req.Currency)
	amountUSD, sourceRateUSD, err := h.toUSD(req.Amount, sourceCurrency)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if amountUSD < 1 || amountUSD > 1000 {
		http.Error(w, "amount must be 1-1000 USD equivalent", http.StatusBadRequest)
		return
	}

	inv, err := h.client.CreateInvoice(CreateInvoiceReq{
		CurrencyType: "fiat",
		Fiat:         "USD",
		Amount:       strconv.FormatFloat(amountUSD, 'f', 2, 64),
		Payload:      user.ID,
		Description:  "Durak Online - balance top up",
		ExpiresIn:    3600,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	url := inv.MiniAppInvoiceURL
	if url == "" {
		url = inv.BotInvoiceURL
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"invoiceId":      inv.InvoiceID,
		"invoiceUrl":     url,
		"amount":         amountUSD,
		"amountUsd":      amountUSD,
		"sourceAmount":   req.Amount,
		"sourceCurrency": sourceCurrency,
		"sourceUsdRate":  sourceRateUSD,
		"status":         inv.Status,
	})
}

func (h *Handler) toUSD(amount float64, currency string) (float64, float64, error) {
	if amount <= 0 {
		return 0, 0, fmt.Errorf("amount must be positive")
	}
	if h.converter == nil {
		converter, err := money.NewConverter("")
		if err != nil {
			return 0, 0, fmt.Errorf("fx converter is not configured")
		}
		h.converter = converter
	}
	return h.converter.ToUSD(amount, currency)
}

func (h *Handler) withdrawFeeBps(method string) int {
	if method == withdrawMethodCard {
		return h.withdrawCardFeeBps
	}
	return h.withdrawCryptoBps
}

func normalizeWithdrawMethod(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "crypto", "usdt", "cryptobot":
		return withdrawMethodCrypto, nil
	case "card", "bank_card", "bank-card":
		return withdrawMethodCard, nil
	default:
		return "", fmt.Errorf("unsupported withdraw method")
	}
}

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Warn("cryptopay webhook: failed to read body")
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	h.log.Info("cryptopay webhook: received",
		zap.Int("body_len", len(body)),
		zap.Bool("has_sig", r.Header.Get("crypto-pay-api-signature") != ""),
	)

	sig := r.Header.Get("crypto-pay-api-signature")
	sigOk := VerifyWebhookSignature(h.token, body, sig)
	if !sigOk {
		h.log.Warn("cryptopay webhook: invalid signature")
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	update, err := ParseWebhookPayload(body)
	if err != nil {
		h.log.Warn("cryptopay webhook: parse failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if update.UpdateType != "invoice_paid" || update.Payload.Status != "paid" {
		h.log.Debug("cryptopay webhook: ignored", zap.String("type", update.UpdateType))
		w.WriteHeader(http.StatusOK)
		return
	}

	userID := update.Payload.Payload
	invoiceID := update.Payload.InvoiceID
	if userID == "" {
		h.log.Warn("cryptopay webhook: empty user payload", zap.Int64("invoice_id", invoiceID))
		w.WriteHeader(http.StatusOK)
		return
	}

	key := "cryptopay:paid:" + strconv.FormatInt(invoiceID, 10)
	ok, err := h.redis.SetNX(r.Context(), key, "1", paidInvoiceTTL).Result()
	if err != nil || !ok {
		if !ok {
			h.log.Debug("cryptopay webhook: duplicate invoice", zap.Int64("invoice_id", invoiceID))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	amountUSD := parsePaidAmountUSD(update.Payload)
	if amountUSD <= 0 {
		h.log.Warn("cryptopay webhook: invalid amount", zap.Int64("invoice_id", invoiceID))
		w.WriteHeader(http.StatusOK)
		return
	}

	_, err = h.txRepo.Add(r.Context(), transactions.Transaction{
		UserID: userID,
		Type:   transactions.TypeDeposit,
		Amount: amountUSD,
		Status: transactions.StatusConfirmed,
	})
	if err != nil {
		h.redis.Del(r.Context(), key)
		h.log.Error("cryptopay webhook: credit failed",
			zap.Int64("invoice_id", invoiceID),
			zap.Error(err),
		)
		http.Error(w, "failed to credit", http.StatusInternalServerError)
		return
	}

	if h.paymentsRepo != nil {
		_ = h.paymentsRepo.InsertCryptoPayPaid(r.Context(), userID, strconv.FormatInt(invoiceID, 10), amountUSD, body)
	}
	h.log.Info("cryptopay webhook: credit success",
		zap.Int64("invoice_id", invoiceID),
		zap.Float64("amount_usd", amountUSD),
	)
	w.WriteHeader(http.StatusOK)
}

func parsePaidAmountUSD(inv Invoice) float64 {
	if inv.CurrencyType == "fiat" && inv.Fiat == "USD" {
		if a, err := strconv.ParseFloat(inv.Amount, 64); err == nil {
			return a
		}
	}
	paid, err1 := strconv.ParseFloat(inv.PaidAmount, 64)
	rate, err2 := strconv.ParseFloat(inv.PaidUsdRate, 64)
	if err1 != nil || err2 != nil {
		return 0
	}
	return paid * rate
}
