package rooms

import (
	"errors"
	"net/http"
	"testing"

	"durakonline/backend/internal/wallet"
)

func TestClassifyRoomGetError(t *testing.T) {
	statusCode, code, message := classifyRoomGetError(ErrRoomNotFound)
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", statusCode)
	}
	if code != "room_not_found" {
		t.Fatalf("expected room_not_found, got %q", code)
	}
	if message != ErrRoomNotFound.Error() {
		t.Fatalf("expected %q, got %q", ErrRoomNotFound.Error(), message)
	}
}

func TestClassifyRoomMutationError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		fallback       string
		wantStatusCode int
		wantCode       string
		wantMessage    string
	}{
		{
			name:           "room full",
			err:            ErrRoomFull,
			fallback:       "failed to join room",
			wantStatusCode: http.StatusBadRequest,
			wantCode:       "room_full",
			wantMessage:    ErrRoomFull.Error(),
		},
		{
			name:           "invalid stake",
			err:            ErrInvalidStake,
			fallback:       "failed to create room",
			wantStatusCode: http.StatusBadRequest,
			wantCode:       "invalid_stake",
			wantMessage:    ErrInvalidStake.Error(),
		},
		{
			name:           "insufficient funds",
			err:            wallet.ErrInsufficientBalance,
			fallback:       "failed to start room",
			wantStatusCode: http.StatusBadRequest,
			wantCode:       "insufficient_funds",
			wantMessage:    wallet.ErrInsufficientBalance.Error(),
		},
		{
			name:           "internal fallback",
			err:            errors.New("save room: redis down"),
			fallback:       "failed to join room",
			wantStatusCode: http.StatusInternalServerError,
			wantCode:       "internal_error",
			wantMessage:    "failed to join room",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusCode, code, message := classifyRoomMutationError(tt.err, tt.fallback)
			if statusCode != tt.wantStatusCode {
				t.Fatalf("expected status %d, got %d", tt.wantStatusCode, statusCode)
			}
			if code != tt.wantCode {
				t.Fatalf("expected code %q, got %q", tt.wantCode, code)
			}
			if message != tt.wantMessage {
				t.Fatalf("expected message %q, got %q", tt.wantMessage, message)
			}
		})
	}
}

func TestValidateStake(t *testing.T) {
	tests := []struct {
		name    string
		stake   float64
		wantErr error
	}{
		{name: "negative", stake: -10, wantErr: ErrInvalidStake},
		{name: "zero", stake: 0, wantErr: ErrInvalidStake},
		{name: "too large", stake: 500.01, wantErr: ErrInvalidStake},
		{name: "valid", stake: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStake(tt.stake)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validateStake(%v) error = %v, want %v", tt.stake, err, tt.wantErr)
			}
		})
	}
}
