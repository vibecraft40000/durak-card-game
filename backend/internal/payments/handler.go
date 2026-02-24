package payments

import (
	"encoding/json"
	"log"
	"net/http"

	"durakonline/backend/pkg/middleware"
)

type Handler struct {
	service *Service
	apiKey  string
}

func NewHandler(service *Service, apiKey string) *Handler {
	return &Handler{service: service, apiKey: apiKey}
}

// CreatePayment handles POST /api/payments/create (JWT required)
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
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

	payLink, externalID, err := h.service.CreateInvoice(r.Context(), user.ID, req.Amount, user.TelegramID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"directPayLink": payLink,
		"externalId":    externalID,
		"amount":        req.Amount,
	})
}

// Webhook handles POST /api/wallet/webhook (no JWT, HMAC verified)
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := ReadBody(r)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	if err := VerifyWalletSignature(r, h.apiKey, body); err != nil {
		log.Printf("wallet webhook: invalid signature: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	events, err := ParseWebhookEvents(body)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	for _, ev := range events {
		if ev.Type != "ORDER_PAID" {
			continue
		}
		if err := h.service.HandleOrderPaid(ctx, ev.Payload, body); err != nil {
			log.Printf("wallet webhook: HandleOrderPaid: %v", err)
			http.Error(w, "payment processing failed", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
