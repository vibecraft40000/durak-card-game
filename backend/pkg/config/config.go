package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                     string
	PprofPort                string
	PostgresURL              string
	RedisURL                 string
	TelegramBotToken         string
	AllowDevTelegramAuth     bool
	StrictInitDataReplay     bool
	DisableMoney             bool
	JWTSecret                string
	AccessTokenTTL           time.Duration
	RefreshTokenTTL          time.Duration
	InitDataMaxAge           time.Duration
	ReplayTTL                time.Duration
	MatchStateTTL            time.Duration
	RoomWaitTimeout          time.Duration
	DisconnectPolicy         string
	WSSyncDiffSkipFinalState bool
	CommissionBps            int
	AllowedOrigin            string
	Env                      string
	CryptoPayEnabled         bool
	CryptoPayAPIToken        string
	CryptoPayTestnet         bool
	FXRatesUSDPerUnit        string
	WithdrawalsEnabled       bool
	WithdrawCardFeeBps       int
	WithdrawCryptoFeeBps     int
	WalletPayEnabled         bool
	WalletPayAPIKey          string
	WalletPayWebhookPath     string
	AdminSecret              string
	AdminNotifyTelegramIDs   string
	RequiredChannelID        string
	SubscriptionChannelLink  string
	envExplicit              bool
}

func Load() Config {
	env, envExplicit := getEnvWithSource("ENV", "development")
	return Config{
		Port:                     getEnv("PORT", "8080"),
		PprofPort:                getEnv("PPROF_PORT", "6060"),
		PostgresURL:              getEnv("POSTGRES_URL", "postgres://durak:durak@localhost:5432/durak?sslmode=disable"),
		RedisURL:                 getEnv("REDIS_URL", "redis://localhost:6379/0"),
		TelegramBotToken:         getEnv("TELEGRAM_BOT_TOKEN", "dev-bot-token"),
		AllowDevTelegramAuth:     getBool("ALLOW_DEV_TELEGRAM_AUTH", false),
		StrictInitDataReplay:     getBool("STRICT_INITDATA_REPLAY", false),
		DisableMoney:             getBool("DISABLE_MONEY", false),
		JWTSecret:                getEnv("JWT_SECRET", "dev-secret"),
		AccessTokenTTL:           getDuration("JWT_ACCESS_TTL", 15*time.Minute),
		RefreshTokenTTL:          getDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		InitDataMaxAge:           getDuration("TELEGRAM_INIT_MAX_AGE", 24*time.Hour),
		ReplayTTL:                getDuration("AUTH_REPLAY_TTL", 24*time.Hour),
		MatchStateTTL:            getDuration("MATCH_STATE_TTL", 2*time.Hour),
		RoomWaitTimeout:          getDuration("ROOM_WAIT_TIMEOUT", 5*time.Minute),
		DisconnectPolicy:         getEnv("DISCONNECT_POLICY", "abandon"),
		WSSyncDiffSkipFinalState: getBool("WS_SYNC_DIFF_SKIP_FINAL_STATE", false),
		CommissionBps:            getInt("COMMISSION_BPS", 300),
		AllowedOrigin:            getEnv("ALLOWED_ORIGIN", "*"),
		CryptoPayEnabled:         getBool("CRYPTO_PAY_ENABLED", true),
		CryptoPayAPIToken:        getEnv("CRYPTO_PAY_API_TOKEN", ""),
		CryptoPayTestnet:         getBool("CRYPTO_PAY_TESTNET", false),
		FXRatesUSDPerUnit:        getEnv("FX_RATES_USD_PER_UNIT", "USD:1,USDT:1,UAH:0.024,RUB:0.011,EUR:1.08"),
		WithdrawalsEnabled:       getBool("WITHDRAWALS_ENABLED", false),
		WithdrawCardFeeBps:       getInt("WITHDRAW_FEE_CARD_BPS", 200),
		WithdrawCryptoFeeBps:     getInt("WITHDRAW_FEE_CRYPTO_BPS", 0),
		WalletPayEnabled:         getBool("WALLET_PAY_ENABLED", false),
		WalletPayAPIKey:          getEnv("WALLET_PAY_API_KEY", ""),
		WalletPayWebhookPath:     getEnv("WALLET_PAY_WEBHOOK_PATH", "/api/wallet/webhook"),
		AdminSecret:              getEnv("ADMIN_SECRET", ""),
		AdminNotifyTelegramIDs:   getEnv("ADMIN_NOTIFY_TELEGRAM_IDS", ""),
		RequiredChannelID:        getEnv("REQUIRED_CHANNEL_ID", ""),
		SubscriptionChannelLink:  getEnv("SUBSCRIPTION_CHANNEL_LINK", ""),
		Env:                      canonicalEnv(env),
		envExplicit:              envExplicit,
	}
}

func (c Config) Validate() error {
	var errs []error

	if !c.envExplicit {
		errs = append(errs, errors.New("ENV must be explicitly set to development, local, test, staging, or production"))
	}

	if c.Env == "" {
		errs = append(errs, errors.New("ENV must not be empty"))
	} else if !isSupportedEnv(c.Env) {
		errs = append(errs, fmt.Errorf("ENV %q is not supported; use development, local, test, staging, or production", c.Env))
	}

	if c.IsLocalDevelopment() {
		return errors.Join(errs...)
	}

	if c.AllowDevTelegramAuth {
		errs = append(errs, errors.New("ALLOW_DEV_TELEGRAM_AUTH must be false outside local development"))
	}
	if err := validateJWTSecret(c.JWTSecret); err != nil {
		errs = append(errs, fmt.Errorf("JWT_SECRET %w", err))
	}
	if err := validateTelegramBotToken(c.TelegramBotToken); err != nil {
		errs = append(errs, fmt.Errorf("TELEGRAM_BOT_TOKEN %w", err))
	}
	if err := validateAllowedOrigin(c.AllowedOrigin); err != nil {
		errs = append(errs, fmt.Errorf("ALLOWED_ORIGIN %w", err))
	}
	if c.CryptoPayEnabled && strings.TrimSpace(c.CryptoPayAPIToken) == "" {
		errs = append(errs, errors.New("CRYPTO_PAY_API_TOKEN must be set outside local development"))
	}
	if c.WalletPayEnabled && strings.TrimSpace(c.WalletPayAPIKey) == "" {
		errs = append(errs, errors.New("WALLET_PAY_API_KEY must be set when WALLET_PAY_ENABLED=true"))
	}

	return errors.Join(errs...)
}

func (c Config) IsLocalDevelopment() bool {
	switch canonicalEnv(c.Env) {
	case "development", "local", "test":
		return true
	default:
		return false
	}
}

func (c Config) IsProductionLike() bool {
	switch canonicalEnv(c.Env) {
	case "production", "staging":
		return true
	default:
		return false
	}
}

func canonicalEnv(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dev", "development":
		return "development"
	case "local":
		return "local"
	case "test":
		return "test"
	case "stage", "staging":
		return "staging"
	case "prod", "production":
		return "production"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func isSupportedEnv(value string) bool {
	switch canonicalEnv(value) {
	case "development", "local", "test", "staging", "production":
		return true
	default:
		return false
	}
}

func validateJWTSecret(secret string) error {
	trimmed := strings.TrimSpace(secret)
	switch {
	case trimmed == "":
		return errors.New("must be set")
	case isInsecureDefault(trimmed, "dev-secret", "change-me", "change-me-in-production"):
		return fmt.Errorf("must not use insecure default value %q", trimmed)
	case len(trimmed) < 16:
		return errors.New("must be at least 16 characters outside local development")
	default:
		return nil
	}
}

func validateTelegramBotToken(token string) error {
	trimmed := strings.TrimSpace(token)
	switch {
	case trimmed == "":
		return errors.New("must be set")
	case isInsecureDefault(trimmed, "dev-bot-token", "change-me", "change-me-in-production"):
		return fmt.Errorf("must not use insecure default value %q", trimmed)
	}

	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return errors.New("must look like a Telegram bot token (<bot_id>:<secret>)")
	}
	if _, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
		return errors.New("must start with a numeric bot id")
	}

	return nil
}

func validateAllowedOrigin(value string) error {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return errors.New("must be set")
	}

	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)
		switch {
		case origin == "":
			return errors.New("must not contain empty origins")
		case origin == "*":
			return errors.New("must not use wildcard outside local development")
		}

		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("must contain valid http(s) origin values, got %q", origin)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("must use http or https origins, got %q", origin)
		}
		if parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("must not include query string or fragment, got %q", origin)
		}
	}

	return nil
}

func isInsecureDefault(value string, defaults ...string) bool {
	for _, candidate := range defaults {
		if value == candidate {
			return true
		}
	}
	return false
}

func getBool(key string, fallback bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getEnvWithSource(key, fallback string) (string, bool) {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value, true
	}
	return fallback, false
}

func getInt(key string, fallback int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getDuration(key string, fallback time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}
