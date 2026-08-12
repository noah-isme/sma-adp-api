package service

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestSMTPPasswordResetEmailDeliverySendsMessage(t *testing.T) {
	messageCh := make(chan string, 1)
	addr := startSMTPTestServer(t, func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		writeSMTPLine(t, conn, "220 smtp.test ESMTP")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				writeSMTPLine(t, conn, "250-smtp.test")
				writeSMTPLine(t, conn, "250 PIPELINING")
			case strings.HasPrefix(line, "MAIL FROM:"):
				writeSMTPLine(t, conn, "250 sender accepted")
			case strings.HasPrefix(line, "RCPT TO:"):
				writeSMTPLine(t, conn, "250 recipient accepted")
			case strings.TrimSpace(line) == "DATA":
				writeSMTPLine(t, conn, "354 end with <CRLF>.<CRLF>")
				var message strings.Builder
				for {
					part, readErr := reader.ReadString('\n')
					if readErr != nil {
						return
					}
					if part == ".\r\n" {
						break
					}
					message.WriteString(part)
				}
				messageCh <- message.String()
				writeSMTPLine(t, conn, "250 queued")
			case strings.TrimSpace(line) == "QUIT":
				writeSMTPLine(t, conn, "221 bye")
				return
			default:
				writeSMTPLine(t, conn, "250 ok")
			}
		}
	})

	delivery, err := NewSMTPPasswordResetEmailDelivery(SMTPPasswordResetEmailConfig{
		Host:    "127.0.0.1",
		Port:    portFromAddress(addr),
		From:    "no-reply@example.test",
		TLSMode: "none",
		Timeout: time.Second,
		Subject: "Reset kata sandi",
	})
	require.NoError(t, err)

	resetURL := "https://admin.example.test/reset-password?token=secret-token"
	expiresAt := time.Date(2026, time.August, 12, 10, 30, 0, 0, time.UTC)
	require.NoError(t, delivery.SendPasswordReset(context.Background(), "admin@example.test", resetURL, expiresAt))

	select {
	case message := <-messageCh:
		assert.Contains(t, message, "From: no-reply@example.test")
		assert.Contains(t, message, "To: admin@example.test")
		assert.Contains(t, message, "Subject: Reset kata sandi")
		assert.Contains(t, message, resetURL)
	case <-time.After(time.Second):
		t.Fatal("SMTP server did not receive message")
	}
}

func TestSMTPPasswordResetEmailDeliveryReturnsSMTPErrorWithoutResetURL(t *testing.T) {
	addr := startSMTPTestServer(t, func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		writeSMTPLine(t, conn, "220 smtp.test ESMTP")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				writeSMTPLine(t, conn, "250 smtp.test")
			case strings.HasPrefix(line, "MAIL FROM:"):
				writeSMTPLine(t, conn, "250 sender accepted")
			case strings.HasPrefix(line, "RCPT TO:"):
				writeSMTPLine(t, conn, "550 recipient rejected")
				return
			}
		}
	})

	delivery, err := NewSMTPPasswordResetEmailDelivery(SMTPPasswordResetEmailConfig{
		Host:    "127.0.0.1",
		Port:    portFromAddress(addr),
		From:    "no-reply@example.test",
		TLSMode: "none",
		Timeout: time.Second,
	})
	require.NoError(t, err)

	resetURL := "https://admin.example.test/reset-password?token=secret-token"
	err = delivery.SendPasswordReset(context.Background(), "admin@example.test", resetURL, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set recipient")
	assert.Contains(t, err.Error(), "550")
	assert.NotContains(t, err.Error(), "secret-token")
}

func TestSMTPPasswordResetEmailDeliveryHonorsContextTimeout(t *testing.T) {
	addr := startSMTPTestServer(t, func(conn net.Conn) {
		// Keep the connection open without sending a greeting. The delivery's
		// context watcher must close it when the timeout expires.
		buffer := make([]byte, 1)
		_, _ = conn.Read(buffer)
	})

	delivery, err := NewSMTPPasswordResetEmailDelivery(SMTPPasswordResetEmailConfig{
		Host:    "127.0.0.1",
		Port:    portFromAddress(addr),
		From:    "no-reply@example.test",
		TLSMode: "none",
		Timeout: 50 * time.Millisecond,
	})
	require.NoError(t, err)

	started := time.Now()
	err = delivery.SendPasswordReset(context.Background(), "admin@example.test", "https://example.test/reset?token=secret", time.Now())
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(started), time.Second)
}

func TestLoggingPasswordResetEmailDeliveryRedactsResetURL(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	delivery := NewLoggingPasswordResetEmailDelivery(zap.New(core))
	resetURL := "https://example.test/reset?token=secret-token"

	require.NoError(t, delivery.SendPasswordReset(context.Background(), "admin@example.test", resetURL, time.Now()))
	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	for _, field := range entry.Context {
		assert.NotEqual(t, "reset_url", field.Key)
		assert.NotContains(t, fmt.Sprint(field.Interface), "secret-token")
	}
}

func TestNewSMTPPasswordResetEmailDeliveryValidatesSettings(t *testing.T) {
	_, err := NewSMTPPasswordResetEmailDelivery(SMTPPasswordResetEmailConfig{
		Host:    "smtp.example.test",
		Port:    587,
		From:    "no-reply@example.test\r\nBcc: attacker@example.test",
		TLSMode: "starttls",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CR or LF")
}

func startSMTPTestServer(t *testing.T, handler func(net.Conn)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		handler(conn)
		_ = conn.Close()
	}()
	return listener.Addr().String()
}

func portFromAddress(address string) int {
	_, portString, err := net.SplitHostPort(address)
	if err != nil {
		panic(err)
	}
	var port int
	if _, err := fmt.Sscanf(portString, "%d", &port); err != nil {
		panic(err)
	}
	return port
}

func writeSMTPLine(t *testing.T, conn net.Conn, line string) {
	t.Helper()
	_, err := fmt.Fprintf(conn, "%s\r\n", line)
	if err != nil {
		return
	}
}
