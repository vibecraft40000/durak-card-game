package payments

import "testing"

func TestParseOrderAmountUSD(t *testing.T) {
	tests := []struct {
		name    string
		amount  MoneyAmount
		want    float64
		wantErr bool
	}{
		{
			name:   "usd ok",
			amount: MoneyAmount{CurrencyCode: "USD", Amount: "12.34"},
			want:   12.34,
		},
		{
			name:    "unsupported currency",
			amount:  MoneyAmount{CurrencyCode: "EUR", Amount: "10"},
			wantErr: true,
		},
		{
			name:    "invalid amount",
			amount:  MoneyAmount{CurrencyCode: "USD", Amount: "abc"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOrderAmountUSD(tc.amount)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got amount=%v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected amount: got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestParseWebhookEvents(t *testing.T) {
	raw := []byte(`[
		{
			"eventDateTime":"2026-02-26T10:00:00Z",
			"eventId":1,
			"type":"ORDER_PAID",
			"payload":{
				"id":101,
				"number":"A-1",
				"externalId":"ext-1",
				"orderAmount":{"currencyCode":"USD","amount":"15.00"}
			}
		}
	]`)

	events, err := ParseWebhookEvents(raw)
	if err != nil {
		t.Fatalf("ParseWebhookEvents returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("unexpected events count: %d", len(events))
	}
	if events[0].Type != "ORDER_PAID" {
		t.Fatalf("unexpected type: %s", events[0].Type)
	}
	if events[0].Payload.ExternalID != "ext-1" {
		t.Fatalf("unexpected external id: %s", events[0].Payload.ExternalID)
	}
}
