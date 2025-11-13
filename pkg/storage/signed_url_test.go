package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSignedURLSignerGenerateAndParse(t *testing.T) {
	signer := NewSignedURLSigner("secret", time.Hour)
	token, expiresAt, err := signer.Generate("job-1", "reports/file.csv")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.False(t, expiresAt.IsZero())

	jobID, path, parsedExpiry, err := signer.Parse(token, false)
	require.NoError(t, err)
	require.Equal(t, "job-1", jobID)
	require.Equal(t, "reports/file.csv", path)
	require.WithinDuration(t, expiresAt, parsedExpiry, time.Second)
}

func TestSignedURLSignerExpired(t *testing.T) {
	signer := NewSignedURLSigner("secret", time.Millisecond*10)
	token, _, err := signer.Generate("job-1", "reports/file.csv")
	require.NoError(t, err)
	time.Sleep(time.Millisecond * 20)

	_, _, _, err = signer.Parse(token, false)
	require.Error(t, err)

	jobID, path, _, err := signer.Parse(token, true)
	require.NoError(t, err)
	require.Equal(t, "job-1", jobID)
	require.Equal(t, "reports/file.csv", path)
}
