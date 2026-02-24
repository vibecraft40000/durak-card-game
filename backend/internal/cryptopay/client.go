package cryptopay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	BaseURL    = "https://pay.crypt.bot"
	TestnetURL = "https://testnet-pay.crypt.bot"
)

type Client struct {
	token   string
	baseURL string
	client  *http.Client
}

func NewClient(token string, testnet bool) *Client {
	base := BaseURL
	if testnet {
		base = TestnetURL
	}
	return &Client{token: token, baseURL: base, client: &http.Client{}}
}

type CreateInvoiceReq struct {
	Asset        string `json:"asset,omitempty"`
	Amount       string `json:"amount"`
	CurrencyType string `json:"currency_type,omitempty"`
	Fiat         string `json:"fiat,omitempty"`
	Payload      string `json:"payload,omitempty"`
	Description  string `json:"description,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
}

type Invoice struct {
	InvoiceID         int64  `json:"invoice_id"`
	Hash              string `json:"hash"`
	Amount            string `json:"amount"`
	Asset             string `json:"asset,omitempty"`
	Fiat              string `json:"fiat,omitempty"`
	CurrencyType      string `json:"currency_type,omitempty"`
	Status            string `json:"status"`
	Payload           string `json:"payload,omitempty"`
	BotInvoiceURL     string `json:"bot_invoice_url"`
	MiniAppInvoiceURL string `json:"mini_app_invoice_url"`
	WebAppInvoiceURL  string `json:"web_app_invoice_url"`
	PaidAmount        string `json:"paid_amount,omitempty"`
	PaidAsset         string `json:"paid_asset,omitempty"`
	PaidUsdRate       string `json:"paid_usd_rate,omitempty"`
}

type APIResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

// parseAPIError extracts a human-readable message from Crypto Pay API error.
// The API may return error as a string or as an object {code, name, message}.
func parseAPIError(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "unknown error"
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var o struct {
		Code    interface{} `json:"code"`
		Name    string      `json:"name"`
		Message string      `json:"message"`
	}
	if err := json.Unmarshal(raw, &o); err == nil {
		switch o.Name {
		case "USER_NOT_FOUND":
			return "Сначала откройте @CryptoTestnetBot → Crypto Pay → создайте кошелёк. Используйте тот же Telegram-аккаунт, с которого входите в игру"
		case "INSUFFICIENT_APP_BALANCE":
			return "Недостаточно средств на балансе приложения"
		case "AMOUNT_TOO_SMALL":
			return "Минимальная сумма вывода: 5 USD"
		}
		if o.Message != "" {
			return o.Message
		}
		if o.Name != "" {
			return o.Name
		}
		if o.Code != nil {
			return fmt.Sprintf("%v", o.Code)
		}
	}
	return string(raw)
}

func (c *Client) CreateInvoice(req CreateInvoiceReq) (Invoice, error) {
	if c.token == "" {
		return Invoice{}, fmt.Errorf("crypto pay api token not configured")
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/createInvoice", bytes.NewReader(body))
	if err != nil {
		return Invoice{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Crypto-Pay-API-Token", c.token)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return Invoice{}, err
	}
	defer resp.Body.Close()

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return Invoice{}, err
	}
	if !apiResp.OK {
		return Invoice{}, fmt.Errorf("crypto pay api: %s", parseAPIError(apiResp.Error))
	}

	var inv Invoice
	if err := json.Unmarshal(apiResp.Result, &inv); err != nil {
		return Invoice{}, err
	}
	return inv, nil
}

// TransferReq parameters for Crypto Pay transfer method.
type TransferReq struct {
	UserID   int64  `json:"user_id"`
	Asset    string `json:"asset"`
	Amount   string `json:"amount"`
	SpendID  string `json:"spend_id"`
	Comment  string `json:"comment,omitempty"`
}

// Transfer represents a completed transfer.
type Transfer struct {
	TransferID int64  `json:"transfer_id"`
	UserID     int64  `json:"user_id"`
	Asset      string `json:"asset"`
	Amount     string `json:"amount"`
	Status     string `json:"status"`
	SpendID    string `json:"spend_id,omitempty"`
}

// Transfer sends coins from app balance to a Telegram user.
func (c *Client) Transfer(req TransferReq) (Transfer, error) {
	if c.token == "" {
		return Transfer{}, fmt.Errorf("crypto pay api token not configured")
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/transfer", bytes.NewReader(body))
	if err != nil {
		return Transfer{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Crypto-Pay-API-Token", c.token)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return Transfer{}, err
	}
	defer resp.Body.Close()

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return Transfer{}, err
	}
	if !apiResp.OK {
		return Transfer{}, fmt.Errorf("crypto pay api: %s", parseAPIError(apiResp.Error))
	}

	var tr Transfer
	if err := json.Unmarshal(apiResp.Result, &tr); err != nil {
		return Transfer{}, err
	}
	return tr, nil
}
