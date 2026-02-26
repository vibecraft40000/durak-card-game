package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	ErrInvalidSignature = errors.New("invalid initData signature")
	ErrExpiredAuthDate  = errors.New("initData auth_date expired")
	ErrMissingHash      = errors.New("initData hash is missing")
)

type TelegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	PhotoURL  string `json:"photo_url"`
}

func ValidateInitData(rawInitData string, botToken string, allowDevAuth bool, maxAge time.Duration, now time.Time) (TelegramUser, string, string, error) {
	values, err := url.ParseQuery(rawInitData)
	if err != nil {
		return TelegramUser{}, "", "", fmt.Errorf("parse initData: %w", err)
	}

	hash := values.Get("hash")
	if hash == "" {
		return TelegramUser{}, "", "", ErrMissingHash
	}
	values.Del("hash")

	authDateRaw := values.Get("auth_date")
	authUnix, err := strconv.ParseInt(authDateRaw, 10, 64)
	if err != nil {
		return TelegramUser{}, "", "", fmt.Errorf("parse auth_date: %w", err)
	}
	authDate := time.Unix(authUnix, 0)
	if now.Sub(authDate) > maxAge {
		return TelegramUser{}, "", "", ErrExpiredAuthDate
	}

	dataCheckString := buildDataCheckString(values)
	// Telegram WebApp validation:
	// secret_key = HMAC_SHA256("WebAppData", bot_token)
	secretMac := hmac.New(sha256.New, []byte("WebAppData"))
	secretMac.Write([]byte(botToken))
	secretKey := secretMac.Sum(nil)

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(dataCheckString))
	computed := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(computed), []byte(hash)) {
		if !(hash == "dev" && (botToken == "dev-bot-token" || allowDevAuth)) {
			return TelegramUser{}, "", "", ErrInvalidSignature
		}
	}

	if hash == "dev" && (botToken == "dev-bot-token" || allowDevAuth) {
		computed = "dev"
	}

	user, err := parseTelegramUser(values.Get("user"))
	if err != nil {
		return TelegramUser{}, "", "", err
	}
	referralCode := ExtractReferralCodeFromStartParam(values.Get("start_param"))

	return user, computed, referralCode, nil
}

func buildDataCheckString(values url.Values) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	rows := make([]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, fmt.Sprintf("%s=%s", key, values.Get(key)))
	}
	return strings.Join(rows, "\n")
}

func parseTelegramUser(raw string) (TelegramUser, error) {
	if raw == "" {
		return TelegramUser{}, errors.New("initData user missing")
	}
	var user TelegramUser
	if err := json.Unmarshal([]byte(raw), &user); err != nil {
		return TelegramUser{}, errors.New("invalid initData user payload")
	}
	if user.ID == 0 {
		return TelegramUser{}, errors.New("telegram id missing")
	}
	return user, nil
}

func ExtractReferralCodeFromStartParam(startParam string) string {
	value := strings.TrimSpace(startParam)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "ref_"):
		value = value[4:]
	case strings.HasPrefix(lower, "ref-"):
		value = value[4:]
	default:
		return ""
	}
	value = strings.TrimSpace(value)
	if len(value) < 3 || len(value) > 64 {
		return ""
	}
	for _, ch := range value {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '-' {
			continue
		}
		return ""
	}
	return strings.ToLower(value)
}
