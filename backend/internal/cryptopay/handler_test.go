package cryptopay

import (
	"testing"

	"durakonline/backend/internal/money"
)

func TestNormalizeWithdrawMethod(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default empty", input: "", want: withdrawMethodCrypto},
		{name: "crypto", input: "crypto", want: withdrawMethodCrypto},
		{name: "usdt alias", input: "usdt", want: withdrawMethodCrypto},
		{name: "card", input: "card", want: withdrawMethodCard},
		{name: "bank card alias", input: "bank-card", want: withdrawMethodCard},
		{name: "unsupported", input: "cash", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeWithdrawMethod(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("method mismatch: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestWithdrawFeeBps(t *testing.T) {
	h := &Handler{withdrawCardFeeBps: 200, withdrawCryptoBps: 0}
	if got := h.withdrawFeeBps(withdrawMethodCard); got != 200 {
		t.Fatalf("card fee mismatch: got %d", got)
	}
	if got := h.withdrawFeeBps(withdrawMethodCrypto); got != 0 {
		t.Fatalf("crypto fee mismatch: got %d", got)
	}
}

func TestToUSD(t *testing.T) {
	converter, err := money.NewConverter("USD:1,UAH:0.025")
	if err != nil {
		t.Fatalf("converter init failed: %v", err)
	}
	h := &Handler{converter: converter}

	amountUSD, rate, err := h.toUSD(100, "UAH")
	if err != nil {
		t.Fatalf("toUSD failed: %v", err)
	}
	if rate != 0.025 {
		t.Fatalf("rate mismatch: got %v", rate)
	}
	if amountUSD != 2.5 {
		t.Fatalf("amount mismatch: got %v", amountUSD)
	}
}
