package service

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// PasswordResetEmailDelivery sends a reset link without coupling the auth
// service to a particular mail provider.
type PasswordResetEmailDelivery interface {
	SendPasswordReset(ctx context.Context, recipient, resetURL string, expiresAt time.Time) error
}

// NoopPasswordResetEmailDelivery is the safe default for tests and
// deployments that have not configured an email provider. It never performs
// network I/O.
type NoopPasswordResetEmailDelivery struct{}

func (NoopPasswordResetEmailDelivery) SendPasswordReset(context.Context, string, string, time.Time) error {
	return nil
}

// LoggingPasswordResetEmailDelivery is intended for local development. It
// never sends a network request and intentionally omits the generated link
// from logs because that link contains a one-time reset token.
type LoggingPasswordResetEmailDelivery struct {
	logger *zap.Logger
}

// NewLoggingPasswordResetEmailDelivery creates a non-network development
// delivery implementation.
func NewLoggingPasswordResetEmailDelivery(logger *zap.Logger) *LoggingPasswordResetEmailDelivery {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &LoggingPasswordResetEmailDelivery{logger: logger}
}

func (d *LoggingPasswordResetEmailDelivery) SendPasswordReset(_ context.Context, recipient, _ string, expiresAt time.Time) error {
	d.logger.Info("password reset email delivery suppressed in development",
		zap.String("recipient", recipient),
		zap.Time("expires_at", expiresAt),
	)
	return nil
}
