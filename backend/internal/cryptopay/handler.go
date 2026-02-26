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
	client         *Client
	txRepo         *transactions.Repository
	paymentsRepo   *payments.Repository
	redis          *redis.Client
	token          string
	webappURL      string
	log            *zap.Logger
	notifyBotToken string
	notifyChatIDs  []int64
}

func NewHandler(token string, testnet bool, txRepo *transactions.Repository, r *redis.Client, webappURL string, log *zap.Logger) *Handler {
	useTestnet := testnet || strings.HasPrefix(token, "test")
	client := NewClient(token, useTestnet)
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{client: client, txRepo: txRepo, paymentsRepo: nil, redis: r, token: token, webappURL: webappURL, log: log}
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

const withdrawLockTTL = 30 * time.Second

// CreateWithdraw: списывает средства с баланса и создаёт заявку на ручной вывод.
// Деньги пользователю отправляет администратор вручную (бот только уведомляет админа).
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
			Amount float64 `json:"amount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Amount < 5 || req.Amount > 1000 {
			http.Error(w, "amount must be 5-1000 USD", http.StatusBadRequest)
			return
		}

		balance, err := walletService.Balance(ctx, user.ID)
		if err != nil {
			h.log.Error("withdraw: balance check failed", zap.Error(err))
			http.Error(w, "failed to check balance", http.StatusInternalServerError)
			return
		}
		if balance < req.Amount {
			http.Error(w, "insufficient balance", http.StatusBadRequest)
			return
		}

		lockKey := "withdraw:lock:" + user.ID
		okLock, err := h.redis.SetNX(ctx, lockKey, "1", withdrawLockTTL).Result()
		if err != nil || !okLock {
			http.Error(w, "withdraw in progress or too many attempts", http.StatusTooManyRequests)
			return
		}
		defer h.redis.Del(ctx, lockKey)

		// Фиксируем транзакцию вывода (баланс уменьшается сразу).
		_, err = h.txRepo.Add(ctx, transactions.Transaction{
			UserID: user.ID,
			Type:   transactions.TypeWithdraw,
			Amount: -req.Amount,
			Status: transactions.StatusConfirmed,
		})
		if err != nil {
			h.log.Error("withdraw: debit failed", zap.Error(err))
			http.Error(w, "failed to process withdraw", http.StatusInternalServerError)
			return
		}

		// Уведомляем админа в Telegram — кто и сколько хочет вывести.
		h.notifyWithdraw(ctx, &user, balance, req.Amount)

		// Отвечаем клиенту, что заявка создана; никаких CryptoPay вызовов.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"amount": req.Amount,
		})
	}
}

func (h *Handler) notifyWithdraw(ctx context.Context, user *users.User, balanceBefore float64, amount float64) {
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
	// Статистика по пополнениям/выводам.
	var deposits, withdrawals float64
	if h.txRepo != nil {
		if d, wtot, err := h.txRepo.StatsForUser(ctx, user.ID); err == nil {
			deposits, withdrawals = d, wtot
		} else {
			h.log.Warn("notify withdraw: stats failed", zap.String("user_id", user.ID), zap.Error(err))
		}
	}
	balanceAfter := balanceBefore - amount

	text := fmt.Sprintf(
		"💰 <b>Заявка на вывод</b>\n\n"+
			"Пользователь: %s\n%sTelegram ID: <code>%d</code>\n"+
			"<a href=\"tg://user?id=%d\">Открыть профиль в Telegram</a>\n\n"+
			"Сумма вывода: <b>%.2f USD</b>\n"+
			"Баланс до вывода: <b>%.2f USD</b>\n"+
			"Баланс после вывода: <b>%.2f USD</b>\n\n"+
			"Всего пополнений: <b>%.2f USD</b>\n"+
			"Всего выводов: <b>%.2f USD</b>",
		displayName, usernameLine, user.TelegramID, user.TelegramID,
		amount, balanceBefore, balanceAfter, deposits, withdrawals,
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
		Amount float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Amount < 1 || req.Amount > 1000 {
		http.Error(w, "amount must be 1-1000 USD", http.StatusBadRequest)
		return
	}

	inv, err := h.client.CreateInvoice(CreateInvoiceReq{
		CurrencyType: "fiat",
		Fiat:         "USD",
		Amount:       strconv.FormatFloat(req.Amount, 'f', 2, 64),
		Payload:      user.ID,
		Description:  "Durak Online — пополнение баланса",
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
		"invoiceId":  inv.InvoiceID,
		"invoiceUrl": url,
		"amount":     req.Amount,
		"status":     inv.Status,
	})
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
