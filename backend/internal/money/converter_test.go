package money

import (
	"errors"
	"testing"
)

func TestNewConverter_DefaultRates(t *testing.T) {
	c, err := NewConverter("")
	if err != nil {
		t.Fatalf("NewConverter default failed: %v", err)
	}

	usd, err := c.USDPerUnit("USD")
	if err != nil {
		t.Fatalf("USD rate missing: %v", err)
	}
	if usd != 1 {
		t.Fatalf("USD rate mismatch: got %v want 1", usd)
	}

	uah, err := c.USDPerUnit("UAH")
	if err != nil {
		t.Fatalf("UAH rate missing: %v", err)
	}
	if uah <= 0 {
		t.Fatalf("UAH rate must be positive, got %v", uah)
	}
}

func TestToUSD_WithCustomRate(t *testing.T) {
	c, err := NewConverter("USD:1, UAH:0.025")
	if err != nil {
		t.Fatalf("NewConverter failed: %v", err)
	}

	amountUSD, rate, err := c.ToUSD(100, "UAH")
	if err != nil {
		t.Fatalf("ToUSD failed: %v", err)
	}
	if rate != 0.025 {
		t.Fatalf("rate mismatch: got %v want %v", rate, 0.025)
	}
	if amountUSD != 2.5 {
		t.Fatalf("amount mismatch: got %v want %v", amountUSD, 2.5)
	}
}

func TestToUSD_UnsupportedCurrency(t *testing.T) {
	c, err := NewConverter("USD:1")
	if err != nil {
		t.Fatalf("NewConverter failed: %v", err)
	}

	_, _, err = c.ToUSD(10, "JPY")
	if !errors.Is(err, ErrUnsupportedCurrency) {
		t.Fatalf("expected unsupported currency error, got %v", err)
	}
}

func TestNewConverter_BadSpec(t *testing.T) {
	_, err := NewConverter("USD=1,UAH=oops")
	if !errors.Is(err, ErrInvalidRateSpec) {
		t.Fatalf("expected invalid spec error, got %v", err)
	}
}
