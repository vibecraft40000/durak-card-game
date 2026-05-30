package cryptopay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"durakonline/backend/internal/users"
	"durakonline/backend/pkg/middleware"
)

func TestCreateWithdrawReturnsForbiddenWhenDisabled(t *testing.T) {
	handler := NewHandler("", true, nil, nil, "", nil)
	user := users.User{
		ID:         "user-1",
		TelegramID: 12345,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/withdraw/create", strings.NewReader(`{"amount":10}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, user))
	rec := httptest.NewRecorder()

	handler.CreateWithdraw(nil)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "withdrawals are disabled during beta until a secure manual review flow is implemented") {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestCreateWithdrawReturnsForbiddenEvenWhenConfiguredEnabled(t *testing.T) {
	handler := NewHandler("", true, nil, nil, "", nil).WithWithdrawalsEnabled(true)
	user := users.User{
		ID:         "user-1",
		TelegramID: 12345,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/withdraw/create", strings.NewReader(`{"amount":10,"currency":"USD","method":"crypto"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, user))
	rec := httptest.NewRecorder()

	handler.CreateWithdraw(nil)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "withdrawals are disabled during beta until a secure manual review flow is implemented") {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestCreateDepositReturnsForbiddenWhenDisabled(t *testing.T) {
	handler := NewHandler("", true, nil, nil, "", nil).WithDepositsEnabled(false)
	user := users.User{ID: "user-1"}

	req := httptest.NewRequest(http.MethodPost, "/api/deposit/create", strings.NewReader(`{"amount":10,"currency":"USD"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, user))
	rec := httptest.NewRecorder()

	handler.CreateDepositInvoice(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "deposits are temporarily unavailable during beta maintenance") {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}
