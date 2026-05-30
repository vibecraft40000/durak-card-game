package ws

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WithServerEventMeta ensures server envelope metadata is present.
// Locale is optional and normalized to a short code (ru|uk|en).
func WithServerEventMeta(event ServerEvent, locale string) ServerEvent {
	return withServerEventMeta(event, locale)
}

func withServerEventMeta(event ServerEvent, locale string) ServerEvent {
	if event.CorrelationID == "" {
		event.CorrelationID = newCorrelationID()
	}
	if event.Locale == "" {
		event.Locale = normalizeLocale(locale)
	}
	return event
}

func newCorrelationID() string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return hex.EncodeToString(raw[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func localeFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if locale := normalizeLocale(r.URL.Query().Get("locale")); locale != "" {
		return locale
	}
	return normalizeLocale(r.Header.Get("Accept-Language"))
}

func normalizeLocale(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if i := strings.IndexAny(value, ",;"); i >= 0 {
		value = value[:i]
	}
	value = strings.TrimSpace(value)
	if i := strings.IndexAny(value, "-_"); i >= 0 {
		value = value[:i]
	}
	switch value {
	case "ru", "uk", "en":
		return value
	default:
		return ""
	}
}
