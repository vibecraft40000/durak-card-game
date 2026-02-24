package payments

import "time"

type PaymentStatus string

const (
	StatusPending   PaymentStatus = "pending"
	StatusPaid      PaymentStatus = "paid"
	StatusFailed    PaymentStatus = "failed"
	StatusExpired   PaymentStatus = "expired"
	StatusRefunded  PaymentStatus = "refunded"
)

type Payment struct {
	ID             string
	UserID         string
	ExternalID     string
	WalletOrderID  string
	AmountUSD      float64
	CurrencyCode   string
	PaidAmount     *float64
	PaidCurrency   string
	Status         PaymentStatus
	CreatedAt      time.Time
	PaidAt         *time.Time
	UpdatedAt      time.Time
	RawWebhook     []byte
}

// CreateOrderReq — WalletPay API create order request
type CreateOrderReq struct {
	Amount              MoneyAmount     `json:"amount"`
	ExternalID          string          `json:"externalId"`
	TimeoutSeconds      int             `json:"timeoutSeconds"`
	Description         string          `json:"description"`
	CustomerTelegramUserID int64        `json:"customerTelegramUserId"`
	ReturnURL           string          `json:"returnUrl,omitempty"`
	FailReturnURL       string          `json:"failReturnUrl,omitempty"`
	AutoConversionCurrency string       `json:"autoConversionCurrency,omitempty"`
}

type MoneyAmount struct {
	CurrencyCode string  `json:"currencyCode"`
	Amount       string  `json:"amount"`
}

// CreateOrderResp — WalletPay API create order response
type CreateOrderResp struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID                 int64  `json:"id"`
		Status             string `json:"status"`
		Number             string `json:"number"`
		DirectPayLink      string `json:"directPayLink"`
		PayLink            string `json:"payLink"`
		Amount             MoneyAmount `json:"amount"`
		CreatedDateTime    string `json:"createdDateTime"`
		ExpirationDateTime string `json:"expirationDateTime"`
	} `json:"data"`
}

// WebhookEvent — WalletPay webhook payload item
type WebhookEvent struct {
	EventDateTime string        `json:"eventDateTime"`
	EventID       int64         `json:"eventId"`
	Type          string        `json:"type"`
	Payload       WebhookPayload `json:"payload"`
}

// WebhookPayload — order data in webhook
type WebhookPayload struct {
	ID                    int64               `json:"id"`
	Number                string              `json:"number"`
	ExternalID            string              `json:"externalId"`
	OrderAmount           MoneyAmount         `json:"orderAmount"`
	SelectedPaymentOption *PaymentOption      `json:"selectedPaymentOption,omitempty"`
	OrderCompletedDateTime string             `json:"orderCompletedDateTime,omitempty"`
}

type PaymentOption struct {
	Amount     MoneyAmount `json:"amount"`
	AmountFee  MoneyAmount `json:"amountFee"`
	AmountNet  MoneyAmount `json:"amountNet"`
	ExchangeRate string    `json:"exchangeRate"`
}
