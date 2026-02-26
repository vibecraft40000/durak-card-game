package money

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

var (
	ErrUnsupportedCurrency = errors.New("unsupported currency")
	ErrInvalidRateSpec     = errors.New("invalid fx rates spec")
)

// DefaultFXRatesUSDPerUnit keeps base conversion rates: 1 unit of currency -> USD.
// All balances and settlements remain in USD.
const DefaultFXRatesUSDPerUnit = "USD:1,USDT:1,UAH:0.024,RUB:0.011,EUR:1.08"

type Converter struct {
	usdPerUnit map[string]float64
}

func NewConverter(spec string) (*Converter, error) {
	rates, err := parseRates(spec)
	if err != nil {
		return nil, err
	}
	return &Converter{usdPerUnit: rates}, nil
}

func (c *Converter) USDPerUnit(currency string) (float64, error) {
	if c == nil {
		return 0, fmt.Errorf("%w: converter is nil", ErrInvalidRateSpec)
	}
	normalized := NormalizeCurrency(currency)
	rate, ok := c.usdPerUnit[normalized]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrUnsupportedCurrency, normalized)
	}
	return rate, nil
}

// ToUSD converts source amount to USD and returns:
// - converted amount (rounded to 2 decimals)
// - conversion rate (USD per 1 source unit)
func (c *Converter) ToUSD(amount float64, currency string) (float64, float64, error) {
	if amount <= 0 {
		return 0, 0, errors.New("amount must be positive")
	}
	rate, err := c.USDPerUnit(currency)
	if err != nil {
		return 0, 0, err
	}
	return Round2(amount * rate), rate, nil
}

func NormalizeCurrency(raw string) string {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return "USD"
	}
	return value
}

func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func parseRates(spec string) (map[string]float64, error) {
	normalizedSpec := strings.TrimSpace(spec)
	if normalizedSpec == "" {
		normalizedSpec = DefaultFXRatesUSDPerUnit
	}

	out := make(map[string]float64)
	parts := strings.FieldsFunc(normalizedSpec, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})

	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		separator := strings.IndexAny(item, ":=")
		if separator <= 0 || separator >= len(item)-1 {
			return nil, fmt.Errorf("%w: bad pair %q", ErrInvalidRateSpec, item)
		}

		currency := NormalizeCurrency(item[:separator])
		rateRaw := strings.TrimSpace(item[separator+1:])
		if !isValidCurrency(currency) {
			return nil, fmt.Errorf("%w: invalid currency %q", ErrInvalidRateSpec, currency)
		}
		rate, err := strconv.ParseFloat(rateRaw, 64)
		if err != nil || rate <= 0 {
			return nil, fmt.Errorf("%w: invalid rate for %s", ErrInvalidRateSpec, currency)
		}
		out[currency] = rate
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("%w: empty map", ErrInvalidRateSpec)
	}
	if _, ok := out["USD"]; !ok {
		out["USD"] = 1
	}
	if _, ok := out["USDT"]; !ok {
		out["USDT"] = 1
	}
	return out, nil
}

func isValidCurrency(currency string) bool {
	if len(currency) < 2 || len(currency) > 10 {
		return false
	}
	for _, r := range currency {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return true
}
