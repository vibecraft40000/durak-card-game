package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                   string
	PprofPort              string
	PostgresURL            string
	RedisURL               string
	TelegramBotToken       string
	AllowDevTelegramAuth   bool
	StrictInitDataReplay   bool
	DisableMoney           bool
	JWTSecret              string
	AccessTokenTTL         time.Duration
	RefreshTokenTTL        time.Duration
	InitDataMaxAge         time.Duration
	ReplayTTL              time.Duration
	MatchStateTTL          time.Duration
	RoomWaitTimeout        time.Duration
	CommissionBps          int
	AllowedOrigin          string
	Env                    string // production, development, test
	CryptoPayAPIToken      string
	CryptoPayTestnet       bool
	FXRatesUSDPerUnit      string
	WithdrawCardFeeBps     int
	WithdrawCryptoFeeBps   int
	WalletPayAPIKey        string
	WalletPayWebhookPath   string
	AdminSecret            string // optional: for GET /admin/stats (X-Admin-Secret)
	AdminNotifyTelegramIDs string // comma-separated Telegram IDs to notify on withdraw (e.g. 5521738246)
}

func Load() Config {
	return Config{
		Port:                   getEnv("PORT", "8080"),
		PprofPort:              getEnv("PPROF_PORT", "6060"),
		PostgresURL:            getEnv("POSTGRES_URL", "postgres://durak:durak@localhost:5432/durak?sslmode=disable"),
		RedisURL:               getEnv("REDIS_URL", "redis://localhost:6379/0"),
		TelegramBotToken:       getEnv("TELEGRAM_BOT_TOKEN", "dev-bot-token"),
		AllowDevTelegramAuth:   getBool("ALLOW_DEV_TELEGRAM_AUTH", false),
		StrictInitDataReplay:   getBool("STRICT_INITDATA_REPLAY", false),
		DisableMoney:           getBool("DISABLE_MONEY", false),
		JWTSecret:              getEnv("JWT_SECRET", "dev-secret"),
		AccessTokenTTL:         getDuration("JWT_ACCESS_TTL", 15*time.Minute),
		RefreshTokenTTL:        getDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		InitDataMaxAge:         getDuration("TELEGRAM_INIT_MAX_AGE", 24*time.Hour),
		ReplayTTL:              getDuration("AUTH_REPLAY_TTL", 24*time.Hour),
		MatchStateTTL:          getDuration("MATCH_STATE_TTL", 2*time.Hour),
		RoomWaitTimeout:        getDuration("ROOM_WAIT_TIMEOUT", 5*time.Minute),
		CommissionBps:          getInt("COMMISSION_BPS", 300),
		AllowedOrigin:          getEnv("ALLOWED_ORIGIN", "*"),
		Env:                    getEnv("ENV", "development"),
		CryptoPayAPIToken:      getEnv("CRYPTO_PAY_API_TOKEN", ""),
		CryptoPayTestnet:       getBool("CRYPTO_PAY_TESTNET", false),
		FXRatesUSDPerUnit:      getEnv("FX_RATES_USD_PER_UNIT", "USD:1,USDT:1,UAH:0.024,RUB:0.011,EUR:1.08"),
		WithdrawCardFeeBps:     getInt("WITHDRAW_FEE_CARD_BPS", 200),
		WithdrawCryptoFeeBps:   getInt("WITHDRAW_FEE_CRYPTO_BPS", 0),
		WalletPayAPIKey:        getEnv("WALLET_PAY_API_KEY", ""),
		WalletPayWebhookPath:   getEnv("WALLET_PAY_WEBHOOK_PATH", "/api/wallet/webhook"),
		AdminSecret:            getEnv("ADMIN_SECRET", ""),
		AdminNotifyTelegramIDs: getEnv("ADMIN_NOTIFY_TELEGRAM_IDS", ""),
	}
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
