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

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, 45*time.Minute, cfg.PasswordReset.TokenTTL)
	require.Equal(t, "https://admin.example.test/reset-password", cfg.PasswordReset.URL)
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
