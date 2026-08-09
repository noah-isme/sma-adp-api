package archives

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignedURL_GenerateAndValidate(t *testing.T) {
	signer := NewHMACSignedURLSigner("test_secret", "/api/v1/archives")
	docID := uuid.New()
	clientIP := "192.168.1.100"

	// 1. Normal valid URL generation and validation
	downloadURLStr, err := signer.GenerateSignedURL(docID, "test_doc.pdf", clientIP, 15*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, downloadURLStr)
	assert.True(t, strings.HasPrefix(downloadURLStr, "/api/v1/archives/"+docID.String()+"/download?token="))

	parsedURL, err := url.Parse(downloadURLStr)
	require.NoError(t, err)
	tokenParam := parsedURL.Query().Get("token")
	require.NotEmpty(t, tokenParam)

	validatedID, err := signer.ValidateSignedURLToken(tokenParam, clientIP)
	assert.NoError(t, err)
	assert.Equal(t, docID, validatedID)

	// 2. IP mismatch validation failure
	_, errIP := signer.ValidateSignedURLToken(tokenParam, "10.0.0.1")
	assert.ErrorIs(t, errIP, ErrIPMismatch)

	// 3. Tampered token validation failure
	tamperedToken := tokenParam + "tampered"
	_, errTampered := signer.ValidateSignedURLToken(tamperedToken, clientIP)
	assert.ErrorIs(t, errTampered, ErrInvalidToken)

	// 4. Malformed token (less or more than 4 parts)
	_, errMalformed := signer.ValidateSignedURLToken("invalid-token-string", clientIP)
	assert.ErrorIs(t, errMalformed, ErrInvalidToken)
}

func TestSignedURL_ExpiredToken(t *testing.T) {
	signer := NewHMACSignedURLSigner("test_secret", "/api/v1/archives")
	docID := uuid.New()
	clientIP := "192.168.1.100"

	exactPastExpires := time.Now().Unix() - 60
	exactSig := signer.computeSignature(docID.String(), clientIP, exactPastExpires)
	exactToken := docID.String() + "|" + strconv.FormatInt(exactPastExpires, 10) + "|" + clientIP + "|" + exactSig

	_, err := signer.ValidateSignedURLToken(exactToken, clientIP)
	assert.ErrorIs(t, err, ErrTokenExpired)
}

func TestSignedURL_DefaultAndMaxTTL(t *testing.T) {
	signer := NewHMACSignedURLSigner("", "")
	docID := uuid.New()

	// Default TTL test
	urlStr1, err := signer.GenerateSignedURL(docID, "file.pdf", "127.0.0.1", 0)
	require.NoError(t, err)
	require.NotEmpty(t, urlStr1)

	// Exceeding 24h TTL test
	urlStr2, err := signer.GenerateSignedURL(docID, "file.pdf", "127.0.0.1", 48*time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, urlStr2)
}
