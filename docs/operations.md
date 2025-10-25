# Cutover & Rollback Runbook

## Overview
This runbook covers the legacy NestJS → Go API traffic migration. It combines feature flag toggles, routing guardrails, parity verification, and rollback instructions. Align the migration window with the Phase 6 plan and ensure monitoring dashboards (Cutover Overview, P0 Drilldown, Cache/DB) are staffed.

## Feature Flags & Environment Controls
| Variable | Description | Default |
| --- | --- | --- |
| `ROUTE_TO_GO` | Routes production traffic to Go API when true. | `false` |
| `SHADOW_TRAFFIC` | Mirrors traffic to Go without impacting responses. | `false` |
| `LEGACY_READONLY` | Forces legacy backend into read-only mode. | `false` |
| `CANARY_PERCENTAGE` | Canary slice routed to Go (0, 10, 50, 100). | `0` |
| `CUTOVER_STAGE_HEADER` | Response header exposing stage. | `X-Cutover-Stage` |
| `CUTOVER_SEGMENT_HEADER` | Response header with client segment metadata. | `X-Client-Segment` |
| `LEGACY_HEALTH_URL` | URL probed by `/internal/ping-legacy`. | `http://localhost:3000/health` |
| `GO_HEALTH_URL` | URL probed by `/internal/ping-go`. | `http://localhost:8080/health` |
| `CUTOVER_HEALTH_TIMEOUT` | Timeout for upstream health probes. | `2s` |

Update `.env` (or the deployment secret) using `make toggle-go true|false`. The helper script flips `ROUTE_TO_GO` and preserves shadow mode for rollback drills.

## Rollout Phases
1. **Shadow Traffic**
   - Enable `SHADOW_TRAFFIC=true`, keep `ROUTE_TO_GO=false`.
   - Monitor `/internal/ping-go` for latency & status parity; 99% schema parity from `make shadow-compare`.
2. **Canary 10%**
   - Set `ROUTE_TO_GO=true`, `CANARY_PERCENTAGE=10`.
   - Validate P0 SLOs (p95 ≤ 250 ms, error ≤ 0.5%).
3. **Canary 50%**
   - Increase `CANARY_PERCENTAGE=50`, freeze schema changes.
   - Daily `make contract-test` & `make shadow-compare`.
4. **Full Cutover 100%**
   - Apply `CANARY_PERCENTAGE=100`, toggle `LEGACY_READONLY=true` after verifying write parity.
   - Promote ingress defaults to Go API; leave shadow probes active for 24 h.

## Verification Checklist
- `make contract-test BASE_URL=https://go.example.com/api/v1`
- `make shadow-compare GO_BASE_URL=https://go.example.com LEGACY_BASE_URL=https://legacy.example.com`
- `/internal/ping-go` and `/internal/ping-legacy` returning HTTP 200 with matching stage metadata.
- Prometheus dashboards show `http_error_rate`, `http_latency_p95_p99`, `cache_hit_ratio`, `db_query_duration`, `5xx_by_route` steady.

## Rollback Procedure
Triggers: error rate > 1% for 15 m, p99 latency > 600 ms for 15 m, or data-integrity regression.

1. `make toggle-go false` (disables `ROUTE_TO_GO`, leaves `SHADOW_TRAFFIC` for diagnostics).
2. Reconfigure ingress/DNS to legacy backend defaults.
3. Disable `SHADOW_TRAFFIC` if Go incidents cascade.
4. Purge cache keys (`auth`, `grades`, `attendance`).
5. Record incident, run tabletop review, and attach logs/metrics snapshots in `docs/decommission.md` appendix.

## Observability & Alerting
- Alerts: `HighErrorRate`, `LatencySLOViolation`, `CacheMissSpike`, `DBSlowQuery`.
- Headers `X-Cutover-Stage` and `X-Client-Segment` appear on every response (see `internal/middleware/cutover.go`).
- `/metrics` exposes `cutover_legacy_health` and `cutover_go_health` duration histograms via `MetricsService` instrumentation.

## Post-Cutover Cleanup (D+14)
- Archive NestJS pipeline, revoke unused secrets, snapshot ingress rules to `ops/archive`.
- Remove `ROUTE_TO_GO` and `SHADOW_TRAFFIC` flags from production env after final audit.
- Update `docs/decommission.md` with audit outcomes and attach analytics deltas.
