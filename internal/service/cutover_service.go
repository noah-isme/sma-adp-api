package service

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/pkg/config"
)

const (
	segmentCookieName = "cutover_segment"
)

// CutoverService coordinates feature flags and health probing for the legacy cutover.
type CutoverService struct {
	cfg     config.CutoverConfig
	metrics *MetricsService
	client  *http.Client
}

// NewCutoverService constructs a CutoverService with sane defaults.
func NewCutoverService(cfg config.CutoverConfig, metrics *MetricsService) *CutoverService {
	timeout := cfg.HealthCheckTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &CutoverService{
		cfg:     cfg,
		metrics: metrics,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Stage determines the current rollout stage based on feature flags.
func (s *CutoverService) Stage() models.CutoverStage {
	if s == nil {
		return models.CutoverStageLegacy
	}

	switch {
	case s.cfg.RouteToGo && (s.cfg.LegacyReadOnly || s.cfg.CanaryPercentage >= 100):
		return models.CutoverStageFull
	case s.cfg.RouteToGo:
		return models.CutoverStageCanary
	case s.cfg.ShadowTraffic:
		return models.CutoverStageShadow
	default:
		return models.CutoverStageLegacy
	}
}

// HeadersForRequest returns observability headers for the supplied request.
func (s *CutoverService) HeadersForRequest(r *http.Request) models.CutoverHeaders {
	if s == nil {
		return models.CutoverHeaders{}
	}

	stageHeader := s.cfg.StageHeader
	if stageHeader == "" {
		stageHeader = "X-Cutover-Stage"
	}
	segmentHeader := s.cfg.ClientSegmentHeader
	if segmentHeader == "" {
		segmentHeader = "X-Client-Segment"
	}

	segment := s.segmentForRequest(r, segmentHeader)

	return models.CutoverHeaders{
		StageHeader:   stageHeader,
		Stage:         s.Stage(),
		SegmentHeader: segmentHeader,
		Segment:       segment,
	}
}

func (s *CutoverService) segmentForRequest(r *http.Request, headerName string) string {
	if r == nil {
		return "unknown"
	}
	if headerName == "" {
		headerName = "X-Client-Segment"
	}
	if value := strings.TrimSpace(r.Header.Get(headerName)); value != "" {
		return value
	}
	if cookie, err := r.Cookie(segmentCookieName); err == nil {
		if trimmed := strings.TrimSpace(cookie.Value); trimmed != "" {
			return trimmed
		}
	}

	source := clientSource(r)
	if source == "" {
		source = r.UserAgent()
	}
	if source == "" {
		source = "fallback"
	}

	sum := sha1.Sum([]byte(source))
	bucket := binary.BigEndian.Uint32(sum[:]) % 100
	return fmt.Sprintf("segment-%02d", bucket)
}

func clientSource(r *http.Request) string {
	if r == nil {
		return ""
	}
	// Prefer header injected by proxies.
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return strings.TrimSpace(host)
}

// PingLegacy probes the configured legacy health endpoint.
func (s *CutoverService) PingLegacy(ctx context.Context) (models.CutoverPingResult, error) {
	return s.ping(ctx, "legacy", s.cfg.LegacyHealthURL)
}

// PingGo probes the Go API health endpoint.
func (s *CutoverService) PingGo(ctx context.Context) (models.CutoverPingResult, error) {
	return s.ping(ctx, "go", s.cfg.GoHealthURL)
}

func (s *CutoverService) ping(ctx context.Context, target, url string) (models.CutoverPingResult, error) {
	result := models.CutoverPingResult{
		Target:       target,
		Stage:        s.Stage(),
		RouteToGo:    s.cfg.RouteToGo,
		Shadow:       s.cfg.ShadowTraffic,
		LegacyLocked: s.cfg.LegacyReadOnly,
		CanaryPct:    s.cfg.CanaryPercentage,
		ObservedAt:   time.Now().UTC(),
	}

	if url == "" {
		err := errors.New("health URL not configured")
		result.Error = err.Error()
		return result, err
	}

	client := s.client
	if client == nil {
		timeout := s.cfg.HealthCheckTimeout
		if timeout <= 0 {
			timeout = 2 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	result.Duration = duration

	statusCode := http.StatusServiceUnavailable
	if err != nil {
		result.Error = err.Error()
	} else {
		defer resp.Body.Close()
		statusCode = resp.StatusCode
		result.StatusCode = resp.StatusCode
		if resp.StatusCode >= http.StatusInternalServerError {
			result.Error = fmt.Sprintf("received status %d", resp.StatusCode)
			err = fmt.Errorf("%s health check failed: %s", target, result.Error)
		}
		result.Reachable = resp.StatusCode < http.StatusInternalServerError
	}

	if s.metrics != nil {
		s.metrics.ObserveHTTPRequest(http.MethodGet, fmt.Sprintf("cutover_%s_health", target), statusCode, duration)
	}

	return result, err
}
