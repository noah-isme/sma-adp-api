package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// SMTPPasswordResetEmailConfig contains the transport and message settings
// required by SMTPPasswordResetEmailDelivery. TLSMode may be "starttls",
// "tls" (implicit TLS), or "none" (intended for local test SMTP servers).
type SMTPPasswordResetEmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	TLSMode  string
	Timeout  time.Duration
	Subject  string
}

// SMTPPasswordResetEmailDelivery sends password-reset links over SMTP. It
// deliberately does not log message contents: the reset URL contains the
// one-time token and must never appear in logs.
type SMTPPasswordResetEmailDelivery struct {
	host     string
	port     int
	username string
	password string
	from     string
	tlsMode  string
	timeout  time.Duration
	subject  string
}

var _ PasswordResetEmailDelivery = (*SMTPPasswordResetEmailDelivery)(nil)

// NewSMTPPasswordResetEmailDelivery validates transport settings once at
// startup so a production process cannot silently accept password-reset
// requests while mail delivery is misconfigured.
func NewSMTPPasswordResetEmailDelivery(cfg SMTPPasswordResetEmailConfig) (*SMTPPasswordResetEmailDelivery, error) {
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return nil, errors.New("smtp host is required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("smtp port must be between 1 and 65535, got %d", cfg.Port)
	}
	if strings.TrimSpace(cfg.From) == "" {
		return nil, errors.New("smtp from address is required")
	}
	if strings.ContainsAny(cfg.From, "\r\n") {
		return nil, errors.New("smtp from address must not contain CR or LF")
	}
	if cfg.Username == "" && cfg.Password != "" {
		return nil, errors.New("smtp username is required when smtp password is set")
	}
	if strings.ContainsAny(cfg.Username, "\r\n") || strings.ContainsAny(cfg.Password, "\r\n") {
		return nil, errors.New("smtp credentials must not contain CR or LF")
	}

	tlsMode := strings.ToLower(strings.TrimSpace(cfg.TLSMode))
	if tlsMode == "" {
		tlsMode = "starttls"
	}
	switch tlsMode {
	case "starttls", "tls", "implicit-tls", "ssl", "none":
		if tlsMode == "implicit-tls" || tlsMode == "ssl" {
			tlsMode = "tls"
		}
	default:
		return nil, fmt.Errorf("unsupported smtp tls mode %q (want starttls, tls, or none)", cfg.TLSMode)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	subject := strings.TrimSpace(cfg.Subject)
	if subject == "" {
		subject = "Atur ulang kata sandi Admin SMA"
	}
	if strings.ContainsAny(subject, "\r\n") {
		return nil, errors.New("smtp subject must not contain CR or LF")
	}

	return &SMTPPasswordResetEmailDelivery{
		host:     host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     strings.TrimSpace(cfg.From),
		tlsMode:  tlsMode,
		timeout:  timeout,
		subject:  subject,
	}, nil
}

// SendPasswordReset sends a plain-text reset message. The operation is bound
// to both the caller's context and the configured timeout; cancellation closes
// the connection so it also interrupts an SMTP server that is not responding.
func (d *SMTPPasswordResetEmailDelivery) SendPasswordReset(ctx context.Context, recipient, resetURL string, expiresAt time.Time) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
	if err := validateSMTPField("recipient", recipient); err != nil {
		return err
	}
	if err := validateSMTPField("reset URL", resetURL); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	conn, err := d.dial(ctx)
	if err != nil {
		return smtpOperationError(ctx, "dial", err)
	}
	stopClose := closeConnectionOnContext(ctx, conn)
	defer stopClose()
	defer conn.Close() //nolint:errcheck // best-effort cleanup after SMTP session

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, err := smtp.NewClient(conn, d.host)
	if err != nil {
		return smtpOperationError(ctx, "open session", err)
	}
	defer client.Close() //nolint:errcheck // Quit below is the normal close path

	if d.tlsMode == "starttls" {
		tlsConfig := d.tlsConfig()
		if err := client.StartTLS(tlsConfig); err != nil {
			return smtpOperationError(ctx, "start TLS", err)
		}
	} else if d.tlsMode == "tls" {
		// Implicit TLS is established before smtp.NewClient in dial().
	}

	if d.username != "" {
		auth := smtp.PlainAuth("", d.username, d.password, d.host)
		if err := client.Auth(auth); err != nil {
			return smtpOperationError(ctx, "authenticate", err)
		}
	}
	if err := client.Mail(d.from); err != nil {
		return smtpOperationError(ctx, "set sender", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return smtpOperationError(ctx, "set recipient", err)
	}

	body := passwordResetMessage(d.from, recipient, d.subject, resetURL, expiresAt)
	writer, err := client.Data()
	if err != nil {
		return smtpOperationError(ctx, "start message", err)
	}
	if _, err := io.WriteString(writer, body); err != nil {
		_ = writer.Close()
		return smtpOperationError(ctx, "write message", err)
	}
	if err := writer.Close(); err != nil {
		return smtpOperationError(ctx, "finish message", err)
	}
	if err := client.Quit(); err != nil {
		return smtpOperationError(ctx, "close session", err)
	}
	return nil
}

func (d *SMTPPasswordResetEmailDelivery) dial(ctx context.Context) (net.Conn, error) {
	address := net.JoinHostPort(d.host, fmt.Sprintf("%d", d.port))
	dialer := &net.Dialer{Timeout: d.timeout}
	if d.tlsMode == "tls" {
		tlsDialer := &tls.Dialer{
			NetDialer: dialer,
			Config:    d.tlsConfig(),
		}
		return tlsDialer.DialContext(ctx, "tcp", address)
	}
	return dialer.DialContext(ctx, "tcp", address)
}

func (d *SMTPPasswordResetEmailDelivery) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName: d.host,
		MinVersion: tls.VersionTLS12,
	}
}

func closeConnectionOnContext(ctx context.Context, conn net.Conn) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

func smtpOperationError(ctx context.Context, operation string, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("smtp %s: %w", operation, ctx.Err())
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return fmt.Errorf("smtp %s: %w", operation, context.DeadlineExceeded)
	}
	return fmt.Errorf("smtp %s: %w", operation, err)
}

func validateSMTPField(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain CR or LF", name)
	}
	return nil
}

func passwordResetMessage(from, recipient, subject, resetURL string, expiresAt time.Time) string {
	return fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\nHalo,\r\n\r\nKami menerima permintaan untuk mengatur ulang kata sandi akun Admin SMA Anda.\r\n\r\nGunakan tautan berikut untuk membuat kata sandi baru:\r\n%s\r\n\r\nTautan ini berlaku sampai %s. Jika Anda tidak meminta pengaturan ulang kata sandi, abaikan email ini.\r\n", from, recipient, subject, resetURL, expiresAt.UTC().Format(time.RFC1123Z))
}
