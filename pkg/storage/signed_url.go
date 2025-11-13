package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// SignedURLSigner creates and validates signed download tokens.
type SignedURLSigner struct {
	secret []byte
	ttl    time.Duration
}

// NewSignedURLSigner constructs a signer with the provided secret and TTL.
func NewSignedURLSigner(secret string, ttl time.Duration) *SignedURLSigner {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &SignedURLSigner{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

// Generate returns a signed token referencing the job and file path.
func (s *SignedURLSigner) Generate(jobID, relPath string) (string, time.Time, error) {
	if jobID == "" || relPath == "" {
		return "", time.Time{}, fmt.Errorf("jobID and relPath required")
	}
	if len(s.secret) == 0 {
		return "", time.Time{}, fmt.Errorf("signing secret missing")
	}
	expiresAt := time.Now().Add(s.ttl)
	encodedPath := base64.RawURLEncoding.EncodeToString([]byte(relPath))
	payload := fmt.Sprintf("%s|%d|%s", jobID, expiresAt.Unix(), encodedPath)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	token := strings.Join([]string{jobID, fmt.Sprintf("%d", expiresAt.Unix()), encodedPath, signature}, ".")
	return token, expiresAt, nil
}

// Parse validates a token and returns the embedded metadata.
// When allowExpired is true, the timestamp check is skipped (used by cleanup routines).
func (s *SignedURLSigner) Parse(token string, allowExpired bool) (jobID, relPath string, expiresAt time.Time, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		return "", "", time.Time{}, fmt.Errorf("invalid token format")
	}
	jobID = parts[0]
	ts := parts[1]
	encodedPath := parts[2]
	signature := parts[3]

	rawPath, err := base64.RawURLEncoding.DecodeString(encodedPath)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("decode path: %w", err)
	}

	expUnix, err := parseUnix(ts)
	if err != nil {
		return "", "", time.Time{}, err
	}
	expiresAt = time.Unix(expUnix, 0)

	payload := fmt.Sprintf("%s|%s|%s", jobID, ts, encodedPath)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return "", "", time.Time{}, fmt.Errorf("invalid token signature")
	}
	if !allowExpired && time.Now().After(expiresAt) {
		return "", "", time.Time{}, fmt.Errorf("token expired")
	}
	return jobID, string(rawPath), expiresAt, nil
}

func parseUnix(raw string) (int64, error) {
	var ts int64
	_, err := fmt.Sscanf(raw, "%d", &ts)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp")
	}
	return ts, nil
}
