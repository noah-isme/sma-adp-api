package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/noah-isme/sma-adp-api/pkg/config"
)

func TestCutoverStage(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.CutoverConfig
		want string
	}{
		{
			name: "legacy default",
			cfg:  config.CutoverConfig{},
			want: "legacy",
		},
		{
			name: "shadow traffic",
			cfg:  config.CutoverConfig{ShadowTraffic: true},
			want: "shadow",
		},
		{
			name: "canary",
			cfg:  config.CutoverConfig{RouteToGo: true, CanaryPercentage: 10},
			want: "canary",
		},
		{
			name: "full cutover by percentage",
			cfg:  config.CutoverConfig{RouteToGo: true, CanaryPercentage: 100},
			want: "full-cutover",
		},
		{
			name: "full cutover by readonly",
			cfg:  config.CutoverConfig{RouteToGo: true, LegacyReadOnly: true, CanaryPercentage: 50},
			want: "full-cutover",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewCutoverService(tt.cfg, nil)
			if got := string(svc.Stage()); got != tt.want {
				t.Fatalf("Stage() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCutoverHeaders(t *testing.T) {
	svc := NewCutoverService(config.CutoverConfig{RouteToGo: true, StageHeader: "X-Test-Stage", ClientSegmentHeader: "X-Test-Segment"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Test-Segment", "segment-42")

	headers := svc.HeadersForRequest(req)
	if headers.StageHeader != "X-Test-Stage" {
		t.Fatalf("unexpected stage header: %s", headers.StageHeader)
	}
	if headers.SegmentHeader != "X-Test-Segment" {
		t.Fatalf("unexpected segment header: %s", headers.SegmentHeader)
	}
	if headers.Segment != "segment-42" {
		t.Fatalf("expected propagated segment, got %s", headers.Segment)
	}
	if headers.Stage != "canary" {
		t.Fatalf("expected canary stage, got %s", headers.Stage)
	}
}

func TestPingSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	svc := NewCutoverService(config.CutoverConfig{GoHealthURL: server.URL, HealthCheckTimeout: time.Second}, nil)
	svc.client = server.Client()

	res, err := svc.PingGo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Reachable {
		t.Fatalf("expected reachable target")
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}
	if res.Duration <= 0 {
		t.Fatalf("expected duration > 0")
	}
}

func TestPingFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	svc := NewCutoverService(config.CutoverConfig{LegacyHealthURL: server.URL, HealthCheckTimeout: time.Second}, nil)
	svc.client = server.Client()

	res, err := svc.PingLegacy(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if res.Reachable {
		t.Fatalf("expected unreachable flag when 5xx returned")
	}
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}
	if res.Error == "" {
		t.Fatalf("expected error message")
	}
}

func TestPingMissingURL(t *testing.T) {
	svc := NewCutoverService(config.CutoverConfig{}, nil)
	if _, err := svc.PingLegacy(context.Background()); err == nil {
		t.Fatalf("expected error for missing URL")
	}
}
