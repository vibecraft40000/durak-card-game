package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mw "github.com/go-chi/chi/v5/middleware"
)

func TestWriteErrorIncludesEnvelopeAndRequestID(t *testing.T) {
	handler := mw.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusBadRequest, "invalid_json", "invalid JSON body", map[string]any{
			"field": "body",
		})
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/telegram", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected application/json content type, got %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Code != "invalid_json" {
		t.Fatalf("expected code invalid_json, got %q", resp.Code)
	}
	if resp.Message != "invalid JSON body" {
		t.Fatalf("expected message invalid JSON body, got %q", resp.Message)
	}
	if resp.RequestID == "" {
		t.Fatal("expected request_id in response body")
	}
	details, ok := resp.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected details object, got %T", resp.Details)
	}
	if details["field"] != "body" {
		t.Fatalf("expected details.field=body, got %v", details["field"])
	}
}
