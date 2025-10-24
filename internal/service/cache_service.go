package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// CacheRepository abstracts persistence for cached payloads.
type CacheRepository interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	DeleteByPattern(ctx context.Context, pattern string) error
}

// CacheService orchestrates cache operations and related metrics.
type CacheService struct {
	repo       CacheRepository
	metrics    *MetricsService
	defaultTTL time.Duration
	logger     *zap.Logger
	enabled    bool
}

// NewCacheService constructs a cache service.
func NewCacheService(repo CacheRepository, metrics *MetricsService, defaultTTL time.Duration, logger *zap.Logger, enabled bool) *CacheService {
	if defaultTTL <= 0 {
		defaultTTL = 10 * time.Minute
	}
	return &CacheService{repo: repo, metrics: metrics, defaultTTL: defaultTTL, logger: logger, enabled: enabled}
}

// Enabled indicates whether caching is active.
func (s *CacheService) Enabled() bool {
	return s != nil && s.enabled && s.repo != nil
}

// Get attempts to retrieve a cached entry. It returns true when the cache was hit.
func (s *CacheService) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	if !s.Enabled() {
		return false, nil
	}
	start := time.Now()
	err := s.repo.Get(ctx, key, dest)
	duration := time.Since(start)
	if err != nil {
		if errors.Is(err, appErrors.ErrCacheMiss) {
			if s.metrics != nil {
				s.metrics.RecordCacheOperation(false, duration)
			}
			return false, nil
		}
		if s.metrics != nil {
			s.metrics.RecordCacheOperation(false, duration)
		}
		if s.logger != nil {
			s.logger.Warn("cache get failed", zap.String("key", key), zap.Error(err))
		}
		return false, err
	}
	if s.metrics != nil {
		s.metrics.RecordCacheOperation(true, duration)
	}
	return true, nil
}

// Set stores the value in cache.
func (s *CacheService) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if !s.Enabled() {
		return nil
	}
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	start := time.Now()
	err := s.repo.Set(ctx, key, value, ttl)
	if s.metrics != nil {
		s.metrics.ObserveCacheWrite(time.Since(start))
	}
	if err != nil && s.logger != nil {
		s.logger.Warn("cache set failed", zap.String("key", key), zap.Error(err))
	}
	return err
}

// Invalidate removes cached values for the provided pattern.
func (s *CacheService) Invalidate(ctx context.Context, pattern string) error {
	if !s.Enabled() {
		return nil
	}
	if err := s.repo.DeleteByPattern(ctx, pattern); err != nil {
		if s.logger != nil {
			s.logger.Warn("cache invalidate failed", zap.String("pattern", pattern), zap.Error(err))
		}
		return err
	}
	return nil
}
