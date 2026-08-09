package archives

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SignedURLSigner interface {
	GenerateSignedURL(docID uuid.UUID, filename string, clientIP string, ttl time.Duration) (string, error)
	ValidateSignedURLToken(tokenString string, clientIP string) (uuid.UUID, error)
}

type HMACSignedURLSigner struct {
	secret  []byte
	baseURL string
}

func NewHMACSignedURLSigner(secret string, baseURL string) *HMACSignedURLSigner {
	if secret == "" {
		secret = "default_archives_secret_key"
	}
	if baseURL == "" {
		baseURL = "/api/v1/archives"
	}
	return &HMACSignedURLSigner{
		secret:  []byte(secret),
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (s *HMACSignedURLSigner) GenerateSignedURL(docID uuid.UUID, filename string, clientIP string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if ttl > 24*time.Hour {
		ttl = 24 * time.Hour
	}
	expiresUnix := time.Now().Add(ttl).Unix()

	signature := s.computeSignature(docID.String(), clientIP, expiresUnix)
	token := fmt.Sprintf("%s|%d|%s|%s", docID.String(), expiresUnix, clientIP, signature)

	downloadURL := fmt.Sprintf("%s/%s/download?token=%s", s.baseURL, docID.String(), url.QueryEscape(token))
	return downloadURL, nil
}

func (s *HMACSignedURLSigner) ValidateSignedURLToken(tokenString string, clientIP string) (uuid.UUID, error) {
	if unescaped, err := url.QueryUnescape(tokenString); err == nil && strings.Contains(unescaped, "|") {
		tokenString = unescaped
	}
	parts := strings.Split(tokenString, "|")
	if len(parts) != 4 {
		return uuid.Nil, ErrInvalidToken
	}

	docIDStr := parts[0]
	expiresUnixStr := parts[1]
	tokenClientIP := parts[2]
	signature := parts[3]

	docID, err := uuid.Parse(docIDStr)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	expiresUnix, err := strconv.ParseInt(expiresUnixStr, 10, 64)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	// 1. HMAC Signature Verification
	expectedSig := s.computeSignature(docIDStr, tokenClientIP, expiresUnix)
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return uuid.Nil, ErrInvalidToken
	}

	// 2. TTL Expiration Check
	if time.Now().Unix() > expiresUnix {
		return uuid.Nil, ErrTokenExpired
	}

	// 3. Client IP Binding Validation
	if tokenClientIP != "" && tokenClientIP != clientIP {
		return uuid.Nil, ErrIPMismatch
	}

	return docID, nil
}

func (s *HMACSignedURLSigner) computeSignature(docID string, clientIP string, expiresUnix int64) string {
	mac := hmac.New(sha256.New, s.secret)
	payload := fmt.Sprintf("%s:%s:%d", docID, clientIP, expiresUnix)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
