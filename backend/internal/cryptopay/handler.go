package cryptopay

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"durakonline/backend/internal/payments"
	"durakonline/backend/internal/transactions"
	"durakonline/backend/internal/wallet"
	"durakonline/backend/pkg/middleware"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// #region agent log
var debugLogMu sync.Mutex

func writeDebugLog(location, message, hypothesisId string, data map[string]interface{}) {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()
	logPath := "debug-d47ef9.log"
	if wd, _ := os.Getwd(); wd != "" {
		if filepath.Base(wd) == "backend" {
			logPath = filepath.Join("..", logPath)
		}
	}
	entry := map[string]interface{}{
		"id": "log_" + strconv.FormatInt(time.Now().UnixMilli(), 10),
		"timestamp": time.Now().UnixMilli(),
		"location":  location,
		"message":   message,
		"data":      data,
		"hypothesisId": hypothesisId,
		"sessionId": "d47ef9",
	}
	raw, _ := json.Marshal(entry)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	f.Write(append(raw, '\n'))
	f.Close()
}

func strOrEmpty(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// #endregion

const paidInvoiceTTL = 30 * 24 * time.Hour

type Handler struct {
	client       *Client
	txRepo       *transactions.Repository
	paymentsRepo *payments.Repository
	redis        *redis.Client
	token        string
	webappURL    string
	log          *zap.Logger
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

const withdrawLockTTL = 30 * time.Second

// CreateWithdraw initiates a Crypto Pay transfer to the user (withdraw from game balance).
func (h *Handler) CreateWithdraw(walletService *wallet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.UserFromContext(r.Context())
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

		balance, err := walletService.Balance(r.Context(), user.ID)
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
		okLock, err := h.redis.SetNX(r.Context(), lockKey, "1", withdrawLockTTL).Result()
		if err != nil || !okLock {
			http.Error(w, "withdraw in progress or too many attempts", http.StatusTooManyRequests)
			return
		}
		defer h.redis.Del(r.Context(), lockKey)

		spendID := uuid.NewString()
		_, err = h.txRepo.Add(r.Context(), transactions.Transaction{
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

		tr, err := h.client.Transfer(TransferReq{
			UserID:  user.TelegramID,
			Asset:   "USDT",
			Amount:  strconv.FormatFloat(req.Amount, 'f', 2, 64),
			SpendID: spendID,
			Comment: "Durak Online — вывод средств",
		})
		if err != nil {
			_, _ = h.txRepo.Add(r.Context(), transactions.Transaction{
				UserID: user.ID,
				Type:   transactions.TypeDeposit,
				Amount: req.Amount,
				Status: transactions.StatusConfirmed,
			})
			h.log.Warn("withdraw: transfer failed, refunded",
				zap.Int64("telegram_id", user.TelegramID),
				zap.String("username", user.Username),
				zap.Error(err))
			http.Error(w, "transfer failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transferId": tr.TransferID,
			"amount":     req.Amount,
			"asset":      tr.Asset,
			"status":     tr.Status,
		})
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
		"invoiceId":     inv.InvoiceID,
		"invoiceUrl":    url,
		"amount":        req.Amount,
		"status":        inv.Status,
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
	// #region agent log
	writeDebugLog("cryptopay/handler.go:Webhook", "webhook received", "A", map[string]interface{}{
		"bodyLen": len(body), "hasSig": r.Header.Get("crypto-pay-api-signature") != "",
	})
	// #endregion

	h.log.Info("cryptopay webhook: received",
		zap.Int("body_len", len(body)),
		zap.Bool("has_sig", r.Header.Get("crypto-pay-api-signature") != ""),
	)

	sig := r.Header.Get("crypto-pay-api-signature")
	sigOk := VerifyWebhookSignature(h.token, body, sig)
	// #region agent log
	writeDebugLog("cryptopay/handler.go:afterSig", "signature verified", "B", map[string]interface{}{
		"sigOk": sigOk, "tokenLen": len(h.token), "sigLen": len(sig),
	})
	// #endregion
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

	// #region agent log
	writeDebugLog("cryptopay/handler.go:afterParse", "parsed webhook", "C", map[string]interface{}{
		"updateType": update.UpdateType, "status": update.Payload.Status,
		"invoiceId": update.Payload.InvoiceID,
	})
	// #endregion
	if update.UpdateType != "invoice_paid" || update.Payload.Status != "paid" {
		h.log.Debug("cryptopay webhook: ignored", zap.String("type", update.UpdateType))
		w.WriteHeader(http.StatusOK)
		return
	}

	userID := update.Payload.Payload
	invoiceID := update.Payload.InvoiceID
	// #region agent log
	writeDebugLog("cryptopay/handler.go:userID", "user payload", "D", map[string]interface{}{
		"userID": userID, "userIDEmpty": userID == "", "invoiceId": invoiceID,
	})
	// #endregion
	if userID == "" {
		h.log.Warn("cryptopay webhook: empty user payload", zap.Int64("invoice_id", invoiceID))
		w.WriteHeader(http.StatusOK)
		return
	}

	key := "cryptopay:paid:" + strconv.FormatInt(invoiceID, 10)
	ok, err := h.redis.SetNX(r.Context(), key, "1", paidInvoiceTTL).Result()
	// #region agent log
	writeDebugLog("cryptopay/handler.go:setnx", "redis SetNX", "H", map[string]interface{}{
		"setnxOk": ok, "redisErr": strOrEmpty(err), "invoiceId": invoiceID,
	})
	// #endregion
	if err != nil || !ok {
		if !ok {
			h.log.Debug("cryptopay webhook: duplicate invoice", zap.Int64("invoice_id", invoiceID))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	amountUSD := parsePaidAmountUSD(update.Payload)
	// #region agent log
	writeDebugLog("cryptopay/handler.go:amountUSD", "parsed amount", "G", map[string]interface{}{
		"amountUSD": amountUSD, "currencyType": update.Payload.CurrencyType,
		"fiat": update.Payload.Fiat, "amount": update.Payload.Amount,
		"paidAmount": update.Payload.PaidAmount, "paidUsdRate": update.Payload.PaidUsdRate,
	})
	// #endregion
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
	// #region agent log
	writeDebugLog("cryptopay/handler.go:txAdd", "txRepo.Add result", "E", map[string]interface{}{
		"addErr": strOrEmpty(err), "userID": userID, "amountUSD": amountUSD,
	})
	// #endregion
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
