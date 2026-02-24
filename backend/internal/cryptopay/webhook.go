package cryptopay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrInvalidSignature = errors.New("invalid crypto pay webhook signature")

type WebhookUpdate struct {
	UpdateType  string  `json:"update_type"`
	UpdateID    int64   `json:"update_id"`
	RequestDate string  `json:"request_date"`
	Payload     Invoice `json:"payload"`
}

func VerifyWebhookSignature(token string, body []byte, signature string) bool {
	if token == "" || signature == "" {
		return false
	}
	secret := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func ParseWebhookPayload(body []byte) (WebhookUpdate, error) {
	var u WebhookUpdate
	if err := json.Unmarshal(body, &u); err != nil {
		return WebhookUpdate{}, fmt.Errorf("parse webhook: %w", err)
	}
	return u, nil
}
