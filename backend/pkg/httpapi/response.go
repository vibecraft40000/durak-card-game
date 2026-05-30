package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	mw "github.com/go-chi/chi/v5/middleware"
)

type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func RequestID(ctx context.Context) string {
	return mw.GetReqID(ctx)
}

func WriteJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string, details any) {
	requestID := RequestID(r.Context())
	if requestID != "" {
		w.Header().Set("X-Request-ID", requestID)
	}
	WriteJSON(w, statusCode, ErrorResponse{
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: requestID,
	})
}
