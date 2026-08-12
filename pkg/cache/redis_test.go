package cache

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/noah-isme/sma-adp-api/pkg/config"
)

func TestRedisOptionsEnableManagedTLS(t *testing.T) {
	options := redisOptions(config.RedisConfig{Host: "redis.example.test", Port: 6379, TLS: true})

	require.NotNil(t, options.TLSConfig)
	require.Equal(t, uint16(tls.VersionTLS12), options.TLSConfig.MinVersion)
	require.Equal(t, "redis.example.test", options.TLSConfig.ServerName)
}

func TestRedisOptionsKeepLocalConnectionsPlain(t *testing.T) {
	options := redisOptions(config.RedisConfig{Host: "localhost", Port: 6379})

	require.Nil(t, options.TLSConfig)
}
