package config

import (
	"strings"
	"testing"
)

func TestValidate_AllowsExplicitLocalDevelopmentDefaults(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("JWT_SECRET", "dev-secret")
	t.Setenv("TELEGRAM_BOT_TOKEN", "dev-bot-token")
	t.Setenv("ALLOWED_ORIGIN", "*")
	t.Setenv("ALLOW_DEV_TELEGRAM_AUTH", "true")

	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error for explicit development config: %v", err)
	}
}

func TestValidate_FailsWithoutExplicitENV(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("TELEGRAM_BOT_TOKEN", "123456:abcdefghijklmnopqrstuvwxyz")
	t.Setenv("ALLOWED_ORIGIN", "https://your-domain.example")

	cfg := Load()
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected error for implicit ENV default")
	}
}

func TestValidate_FailsInProductionWithDevFallbacks(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "dev-secret")
	t.Setenv("TELEGRAM_BOT_TOKEN", "dev-bot-token")
	t.Setenv("ALLOWED_ORIGIN", "*")
	t.Setenv("ALLOW_DEV_TELEGRAM_AUTH", "true")

	cfg := Load()
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for insecure production config")
	}

	msg := err.Error()
	for _, want := range []string{
		"ALLOW_DEV_TELEGRAM_AUTH must be false outside local development",
		`JWT_SECRET must not use insecure default value "dev-secret"`,
		`TELEGRAM_BOT_TOKEN must not use insecure default value "dev-bot-token"`,
		"ALLOWED_ORIGIN must not use wildcard outside local development",
	} {
		if !contains(msg, want) {
			t.Fatalf("Validate() error %q does not contain %q", msg, want)
		}
	}
}

func TestValidate_FailsInStagingWithWeakSecrets(t *testing.T) {
	t.Setenv("ENV", "staging")
	t.Setenv("JWT_SECRET", "short-secret")
	t.Setenv("TELEGRAM_BOT_TOKEN", "not-a-bot-token")
	t.Setenv("ALLOWED_ORIGIN", "https://your-domain.example")

	cfg := Load()
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for weak staging secrets")
	}

	msg := err.Error()
	if !contains(msg, "JWT_SECRET must be at least 16 characters outside local development") {
		t.Fatalf("Validate() error %q does not contain JWT_SECRET length failure", msg)
	}
	if !contains(msg, "TELEGRAM_BOT_TOKEN must look like a Telegram bot token") {
		t.Fatalf("Validate() error %q does not contain TELEGRAM_BOT_TOKEN format failure", msg)
	}
}

func TestValidate_AllowsProductionWithExplicitSecureConfig(t *testing.T) {
	t.Setenv("ENV", "prod")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("TELEGRAM_BOT_TOKEN", "123456:abcdefghijklmnopqrstuvwxyz")
	t.Setenv("ALLOWED_ORIGIN", "https://your-domain.example,https://staging.your-domain.example")

	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error for secure production config: %v", err)
	}
	if cfg.Env != "production" {
		t.Fatalf("expected ENV alias to normalize to production, got %q", cfg.Env)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
