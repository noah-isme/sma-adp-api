package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadParsesPasswordResetSMTPSettings(t *testing.T) {
	t.Setenv("ENV", EnvProduction)
	t.Setenv("PASSWORD_RESET_TOKEN_TTL", "45m")
	t.Setenv("PASSWORD_RESET_URL", "https://admin.example.test/reset-password")
	t.Setenv("PORTAL_PASSWORD_RESET_URL", "https://portal.school.id/portal/reset-password")
	t.Setenv("PASSWORD_RESET_EMAIL_SUBJECT", "Reset kata sandi")
	t.Setenv("SMTP_ENABLED", "true")
	t.Setenv("SMTP_HOST", "smtp.example.test")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USER", "mailer@example.test")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("SMTP_FROM", "no-reply@example.test")
	t.Setenv("SMTP_TLS_MODE", "tls")
	t.Setenv("SMTP_TIMEOUT", "3s")
	t.Setenv("ALLOWED_ORIGINS", "https://admin.example.test")
	t.Setenv("REDIS_TLS", "true")
	t.Setenv("RATE_LIMIT_REQUESTS_PER_MINUTE", "600")
	t.Setenv("RATE_LIMIT_BURST", "120")
	t.Setenv("RATE_LIMIT_MAX_CLIENTS", "2500")

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, 45*time.Minute, cfg.PasswordReset.TokenTTL)
	require.Equal(t, "https://admin.example.test/reset-password", cfg.PasswordReset.URL)
	require.Equal(t, "https://portal.school.id/portal/reset-password", cfg.PasswordReset.PortalURL)
	require.Equal(t, "Reset kata sandi", cfg.PasswordReset.Subject)
	require.Equal(t, SMTPConfig{
		Enabled:  true,
		Host:     "smtp.example.test",
		Port:     2525,
		Username: "mailer@example.test",
		Password: "secret",
		From:     "no-reply@example.test",
		TLSMode:  "tls",
		Timeout:  3 * time.Second,
	}, cfg.SMTP)
	require.Equal(t, []string{"https://admin.example.test"}, cfg.CORS.AllowedOrigins)
	require.True(t, cfg.Redis.TLS)
	require.Equal(t, RateLimitConfig{RequestsPerMinute: 600, Burst: 120, MaxClients: 2500}, cfg.RateLimit)
}

func TestValidateProductionRejectsUnsafePortalResetURL(t *testing.T) {
	base := &Config{Env: EnvProduction}

	for _, value := range []string{"", "http://portal.school.id/reset", "https://localhost/reset", "https://portal.example.com/reset"} {
		base.PasswordReset.PortalURL = value
		require.Error(t, ValidateProduction(base), value)
	}
}

func TestValidateProductionAcceptsHTTPSPortalResetURL(t *testing.T) {
	cfg := &Config{
		Env:           EnvProduction,
		PasswordReset: PasswordResetConfig{PortalURL: "https://portal.school.id/reset-password"},
	}
	require.NoError(t, ValidateProduction(cfg))
}

func TestLoadUsesSafeSMTPDefaults(t *testing.T) {
	for _, key := range []string{
		"SMTP_ENABLED", "SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_USERNAME",
		"SMTP_PASSWORD", "SMTP_FROM", "SMTP_TLS_MODE", "SMTP_TIMEOUT",
	} {
		t.Setenv(key, "")
	}

	cfg, err := Load()
	require.NoError(t, err)

	require.False(t, cfg.SMTP.Enabled)
	require.Empty(t, cfg.SMTP.Host)
	require.Equal(t, 587, cfg.SMTP.Port)
	require.Equal(t, "starttls", cfg.SMTP.TLSMode)
	require.Equal(t, 10*time.Second, cfg.SMTP.Timeout)
}

func TestLoadAcceptsSMTPUsernameAlias(t *testing.T) {
	t.Setenv("SMTP_USER", "")
	t.Setenv("SMTP_USERNAME", "alias@example.test")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "alias@example.test", cfg.SMTP.Username)
}
