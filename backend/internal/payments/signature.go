package payments

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
)

// VerifyWalletSignature verifies Walletpay-Signature against HMAC-SHA256(Method.URI-path.Timestamp.Base64(body)).
// Per docs: Base64(HmacSHA256("HTTP-method.URI-path.timestamp.Base-64-encoded-body")) with Wpay-Store-Api-Key.
func VerifyWalletSignature(r *http.Request, apiKey string, body []byte) error {
	timestamp := r.Header.Get("WalletPay-Timestamp")
	signature := r.Header.Get("Walletpay-Signature")
	if signature == "" {
		signature = r.Header.Get("WalletPay-Signature")
	}
	if timestamp == "" || signature == "" {
		return errors.New("missing WalletPay-Timestamp or Walletpay-Signature headers")
	}

	path := r.URL.Path
	if !strings.HasSuffix(path, "/") && r.URL.RawQuery != "" {
		path = path + "?" + r.URL.RawQuery
	}
	text := strings.Join([]string{
		r.Method,
		path,
		timestamp,
		base64.StdEncoding.EncodeToString(body),
	}, ".")

	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(text))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("invalid WalletPay signature")
	}
	return nil
}

// ReadBody reads request body and restores it for later handlers
func ReadBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
