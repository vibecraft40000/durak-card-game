package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const walletPayAPIBase = "https://pay.wallet.tg/wpay/store-api/v1"

type Client struct {
	apiKey  string
	client  *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// CreateOrder creates an order in WalletPay and returns directPayLink and order ID.
func (c *Client) CreateOrder(ctx context.Context, req CreateOrderReq) (directPayLink string, orderID int64, err error) {
	if c.apiKey == "" {
		return "", 0, fmt.Errorf("WalletPay API key not configured")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, walletPayAPIBase+"/order", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	httpReq.Header.Set("Wpay-Store-Api-Key", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	var out CreateOrderResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", 0, fmt.Errorf("decode WalletPay response: %w", err)
	}
	if out.Status != "SUCCESS" {
		return "", 0, fmt.Errorf("WalletPay API error: %s - %s", out.Status, out.Message)
	}

	link := out.Data.DirectPayLink
	if link == "" {
		link = out.Data.PayLink
	}
	return link, out.Data.ID, nil
}
