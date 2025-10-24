package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// CacheRepository provides helpers around Redis interactions for caching analytics payloads.
type CacheRepository struct {
	client *redis.Client
	logger *zap.Logger
}

// NewCacheRepository constructs a cache repository.
func NewCacheRepository(client *redis.Client, logger *zap.Logger) *CacheRepository {
	return &CacheRepository{client: client, logger: logger}
}

// Get retrieves and unmarshals the cached value into the provided destination.
func (r *CacheRepository) Get(ctx context.Context, key string, dest interface{}) error {
	if r.client == nil {
		return appErrors.ErrCacheMiss
	}

	raw, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return appErrors.ErrCacheMiss
		}
		return fmt.Errorf("redis get %s: %w", key, err)
	}

	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("unmarshal cache value for %s: %w", key, err)
	}

	return nil
}

// Set marshals the provided value and stores it with the given TTL.
func (r *CacheRepository) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if r.client == nil {
		return nil
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal cache value for %s: %w", key, err)
	}

	if err := r.client.Set(ctx, key, payload, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %s: %w", key, err)
	}

	return nil
}

// DeleteByPattern removes cached entries matching the provided pattern.
func (r *CacheRepository) DeleteByPattern(ctx context.Context, pattern string) error {
	if r.client == nil {
		return nil
	}

	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if err := r.client.Del(ctx, key).Err(); err != nil {
			return fmt.Errorf("redis delete %s: %w", key, err)
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("redis scan pattern %s: %w", pattern, err)
	}

	return nil
}

// Close releases the underlying Redis connection if present.
func (r *CacheRepository) Close() error {
	if r.client == nil {
		return nil
	}
	return r.client.Close()
}
