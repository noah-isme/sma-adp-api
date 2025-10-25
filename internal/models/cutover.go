package models

import "time"

// CutoverStage enumerates the rollout phases used during the legacy decommission.
type CutoverStage string

const (
        // CutoverStageLegacy indicates the legacy NestJS backend still serves all traffic.
        CutoverStageLegacy CutoverStage = "legacy"
        // CutoverStageShadow mirrors traffic to the Go API without impacting responses.
        CutoverStageShadow CutoverStage = "shadow"
        // CutoverStageCanary represents partial routing to the Go API (10-50%).
        CutoverStageCanary CutoverStage = "canary"
        // CutoverStageFull indicates 100% routing to the Go API with legacy in read-only mode.
        CutoverStageFull CutoverStage = "full-cutover"
)

// CutoverHeaders captures header metadata applied for observability.
type CutoverHeaders struct {
        StageHeader   string       `json:"stage_header"`
        Stage         CutoverStage `json:"stage"`
        SegmentHeader string       `json:"segment_header"`
        Segment       string       `json:"segment"`
}

// CutoverPingResult describes the outcome of pinging an upstream (legacy or Go).
type CutoverPingResult struct {
        Target       string        `json:"target"`
        Reachable    bool          `json:"reachable"`
        Stage        CutoverStage  `json:"stage"`
        StatusCode   int           `json:"status_code"`
        Duration     time.Duration `json:"duration"`
        ObservedAt   time.Time     `json:"observed_at"`
        Error        string        `json:"error,omitempty"`
        RouteToGo    bool          `json:"route_to_go"`
        Shadow       bool          `json:"shadow"`
        LegacyLocked bool          `json:"legacy_readonly"`
        CanaryPct    int           `json:"canary_percentage"`
}
