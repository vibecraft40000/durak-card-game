package logger

import (
	"net/http"

	"durakonline/backend/pkg/httpapi"

	"go.uber.org/zap"
)

func New() (*zap.Logger, error) {
	return zap.NewProduction()
}

func WithRequest(log *zap.Logger, r *http.Request) *zap.Logger {
	if log == nil {
		log = zap.NewNop()
	}

	fields := []zap.Field{
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	}
	if requestID := httpapi.RequestID(r.Context()); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}

	return log.With(fields...)
}
