package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
	"github.com/noah-isme/sma-adp-api/pkg/response"
)

// RateLimiterConfig configures the in-process token bucket used as a
// defense-in-depth limit behind the edge proxy. The edge still owns the
// primary distributed limit; this limiter protects a directly reachable API
// instance and keeps deployments safe when the proxy is bypassed.
type RateLimiterConfig struct {
	RequestsPerMinute int
	Burst             int
	MaxClients        int
}

type rateLimitBucket struct {
	tokens float64
	last   time.Time
}

// RateLimiter is a per-client token-bucket limiter. It is safe for concurrent
// requests and bounds stale client state so an attacker cannot grow the map
// without limit by rotating source addresses.
type RateLimiter struct {
	mu        sync.Mutex
	rate      float64
	burst     float64
	max       int
	clients   map[string]rateLimitBucket
	now       func() time.Time
	lastSweep time.Time
}

// NewRateLimiter constructs a Gin middleware. Non-positive values use a
// conservative API default of 120 requests/minute with a burst of 60.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	if cfg.RequestsPerMinute <= 0 {
		cfg.RequestsPerMinute = 120
	}
	if cfg.Burst <= 0 {
		cfg.Burst = cfg.RequestsPerMinute / 2
		if cfg.Burst < 1 {
			cfg.Burst = 1
		}
	}
	if cfg.MaxClients <= 0 {
		cfg.MaxClients = 10000
	}
	return &RateLimiter{
		rate:    float64(cfg.RequestsPerMinute) / 60,
		burst:   float64(cfg.Burst),
		max:     cfg.MaxClients,
		clients: make(map[string]rateLimitBucket),
		now:     time.Now,
	}
}

// Handler returns the Gin middleware for this limiter.
func (l *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rateLimitClientKey(c)
		allowed, remaining, retryAfter := l.allow(key)
		c.Header("X-RateLimit-Limit", strconv.Itoa(int(l.burst)))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			response.Error(c, appErrors.New("RATE_LIMITED", http.StatusTooManyRequests, "rate limit exceeded"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimiterMiddleware is a convenience constructor for direct r.Use use.
func RateLimiterMiddleware(cfg RateLimiterConfig) gin.HandlerFunc {
	return NewRateLimiter(cfg).Handler()
}

func (l *RateLimiter) allow(key string) (bool, int, int) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastSweep.IsZero() || now.Sub(l.lastSweep) >= time.Minute {
		for client, bucket := range l.clients {
			if now.Sub(bucket.last) > time.Minute*2 {
				delete(l.clients, client)
			}
		}
		l.lastSweep = now
	}
	if _, ok := l.clients[key]; !ok && len(l.clients) >= l.max {
		// Evict one stale entry before refusing to allocate more state. The
		// normal sweep above keeps this path rare and deterministic.
		for client, bucket := range l.clients {
			if now.Sub(bucket.last) > time.Minute {
				delete(l.clients, client)
				break
			}
		}
	}

	bucket, ok := l.clients[key]
	if !ok {
		bucket = rateLimitBucket{tokens: l.burst, last: now}
	}
	if elapsed := now.Sub(bucket.last).Seconds(); elapsed > 0 {
		bucket.tokens += elapsed * l.rate
		if bucket.tokens > l.burst {
			bucket.tokens = l.burst
		}
		bucket.last = now
	}
	if bucket.tokens < 1 {
		l.clients[key] = bucket
		wait := int((1 - bucket.tokens) / l.rate)
		if wait < 1 {
			wait = 1
		}
		return false, 0, wait
	}
	bucket.tokens--
	remaining := int(bucket.tokens)
	l.clients[key] = bucket
	return true, remaining, 0
}

func rateLimitClientKey(c *gin.Context) string {
	// Nginx canonicalises this header from the trusted Cloudflare edge. Ignore
	// malformed values so a caller cannot spoof arbitrary buckets directly.
	if forwarded := strings.TrimSpace(c.GetHeader("CF-Connecting-IP")); net.ParseIP(forwarded) != nil {
		return forwarded
	}
	if remote := c.RemoteIP(); remote != "" {
		return remote
	}
	return "unknown"
}
