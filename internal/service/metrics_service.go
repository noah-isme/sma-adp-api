package service

import (
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// MetricsService encapsulates Prometheus instrumentation and provides lightweight snapshots for API consumption.
type MetricsService struct {
	registry        *prometheus.Registry
	handler         http.Handler
	requestDuration *prometheus.HistogramVec
	requestTotal    *prometheus.CounterVec
	cacheLatency    prometheus.Observer
	cacheWrite      prometheus.Observer
	cacheHitRatio   prometheus.Gauge
	cacheHits       prometheus.Counter
	cacheMisses     prometheus.Counter
	dbQueryDuration *prometheus.HistogramVec

	cacheHitCount        uint64
	cacheMissCount       uint64
	requestCount         uint64
	requestDurationTotal uint64
	dbQueryCount         uint64
	dbQueryDurationTotal uint64
}

// NewMetricsService registers core Prometheus collectors.
func NewMetricsService() *MetricsService {
	registry := prometheus.NewRegistry()

	requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	requestTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	cacheLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "cache_latency_seconds",
		Help:    "Latency for cache operations",
		Buckets: prometheus.DefBuckets,
	})

	cacheWrite := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "cache_write_seconds",
		Help:    "Latency for cache set operations",
		Buckets: prometheus.DefBuckets,
	})

	cacheHitRatio := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cache_hit_ratio",
		Help: "Ratio of cache hits to total cache lookups",
	})

	cacheHits := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Total cache hits",
	})

	cacheMisses := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Total cache misses",
	})

	dbQueryDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "db_query_duration_seconds",
		Help:    "Duration of database queries",
		Buckets: prometheus.DefBuckets,
	}, []string{"query"})

	goroutines := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "goroutines_total",
		Help: "Total number of goroutines",
	}, func() float64 {
		return float64(runtime.NumGoroutine())
	})

	registry.MustRegister(requestDuration, requestTotal, cacheLatency, cacheWrite, cacheHitRatio, cacheHits, cacheMisses, dbQueryDuration, goroutines)

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	return &MetricsService{
		registry:        registry,
		handler:         handler,
		requestDuration: requestDuration,
		requestTotal:    requestTotal,
		cacheLatency:    cacheLatency,
		cacheWrite:      cacheWrite,
		cacheHitRatio:   cacheHitRatio,
		cacheHits:       cacheHits,
		cacheMisses:     cacheMisses,
		dbQueryDuration: dbQueryDuration,
	}
}

// Handler exposes the Prometheus HTTP handler.
func (m *MetricsService) Handler() http.Handler {
	if m == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})
	}
	return m.handler
}

// ObserveHTTPRequest records request metrics and aggregates simple stats for snapshots.
func (m *MetricsService) ObserveHTTPRequest(method, path string, status int, duration time.Duration) {
	if m == nil {
		return
	}
	labelStatus := fmt.Sprintf("%d", status)
	m.requestDuration.WithLabelValues(method, path, labelStatus).Observe(duration.Seconds())
	m.requestTotal.WithLabelValues(method, path, labelStatus).Inc()
	atomic.AddUint64(&m.requestCount, 1)
	atomic.AddUint64(&m.requestDurationTotal, uint64(duration.Nanoseconds()))
}

// RecordCacheOperation records cache hit/miss metrics and updates hit ratio.
func (m *MetricsService) RecordCacheOperation(hit bool, duration time.Duration) {
	if m == nil {
		return
	}
	if m.cacheLatency != nil {
		m.cacheLatency.Observe(duration.Seconds())
	}
	if hit {
		m.cacheHits.Inc()
		atomic.AddUint64(&m.cacheHitCount, 1)
	} else {
		m.cacheMisses.Inc()
		atomic.AddUint64(&m.cacheMissCount, 1)
	}
	hits := atomic.LoadUint64(&m.cacheHitCount)
	misses := atomic.LoadUint64(&m.cacheMissCount)
	total := hits + misses
	if total > 0 {
		m.cacheHitRatio.Set(float64(hits) / float64(total))
	}
}

// ObserveCacheWrite tracks the duration for cache write operations.
func (m *MetricsService) ObserveCacheWrite(duration time.Duration) {
	if m == nil || m.cacheWrite == nil {
		return
	}
	m.cacheWrite.Observe(duration.Seconds())
}

// ObserveDBQuery records database query timing.
func (m *MetricsService) ObserveDBQuery(label string, duration time.Duration) {
	if m == nil {
		return
	}
	m.dbQueryDuration.WithLabelValues(label).Observe(duration.Seconds())
	atomic.AddUint64(&m.dbQueryCount, 1)
	atomic.AddUint64(&m.dbQueryDurationTotal, uint64(duration.Nanoseconds()))
}

// Snapshot returns aggregated metrics suitable for analytics endpoints.
func (m *MetricsService) Snapshot() models.AnalyticsSystemMetrics {
	if m == nil {
		return models.AnalyticsSystemMetrics{}
	}
	hits := atomic.LoadUint64(&m.cacheHitCount)
	misses := atomic.LoadUint64(&m.cacheMissCount)
	requests := atomic.LoadUint64(&m.requestCount)
	reqDuration := atomic.LoadUint64(&m.requestDurationTotal)
	dbCount := atomic.LoadUint64(&m.dbQueryCount)
	dbDuration := atomic.LoadUint64(&m.dbQueryDurationTotal)

	var cacheRatio float64
	totalLookups := hits + misses
	if totalLookups > 0 {
		cacheRatio = float64(hits) / float64(totalLookups)
	}

	var avgRequestMs float64
	if requests > 0 {
		avgRequestMs = float64(reqDuration) / float64(requests) / float64(time.Millisecond)
	}

	var avgDBMs float64
	if dbCount > 0 {
		avgDBMs = float64(dbDuration) / float64(dbCount) / float64(time.Millisecond)
	}

	return models.AnalyticsSystemMetrics{
		CacheHitRatio:            cacheRatio,
		CacheHits:                hits,
		CacheMisses:              misses,
		RequestsTotal:            requests,
		AverageRequestDurationMs: avgRequestMs,
		DBQueryCount:             dbCount,
		AverageDBQueryDurationMs: avgDBMs,
		Goroutines:               runtime.NumGoroutine(),
		GeneratedAt:              time.Now().UTC(),
	}
}
