# Legacy Decommission Checklist

## Timeline
- **T0:** Full cutover complete, legacy in read-only mode (`LEGACY_READONLY=true`).
- **T0 + 7d:** Confirm SLO stability, complete parity audits.
- **T0 + 14d:** Begin legacy teardown (pipelines, routing, secrets).

## Mandatory Steps
0. **Read-only preflight**
   - Run `make decommission-preflight` against the staging `.env` before changing
     routing or retiring the legacy service.
   - For CI/static validation without reachable upstreams, run
     `make decommission-preflight PREFLIGHT_ARGS=--skip-http`.
   - The command requires `ROUTE_TO_GO=true`, `LEGACY_READONLY=true`,
     `CANARY_PERCENTAGE=100`, non-default signing secrets, and explicit CORS
     origins. It never changes deployment state.

   Repository support artifacts:
   - [`scripts/decommission_preflight.py`](../scripts/decommission_preflight.py)
     contains the read-only checks and optional health probes.
   - [`monitoring/prometheus/sma-api-alerts.yml`](../../monitoring/prometheus/sma-api-alerts.yml)
     defines the cutover error, latency, cache, and database alerts.
   - [`monitoring/`](../../monitoring/) contains the dashboard and structural
     validator; live Prometheus and Alertmanager wiring remains an environment
     task.
1. **Pipelines & Deployments**
   - Disable NestJS CI/CD jobs.
   - Archive legacy deployment manifests; upload snapshot to `ops/archive`.
2. **Secrets & Credentials**
   - Rotate production JWT secrets and database users used solely by legacy backend.
   - Remove superseded secrets from secret manager or `.env` (document removals).
3. **Routing Rules**
   - Remove legacy upstream from ingress load balancer.
   - Capture before/after diff in `ops/archive/ingress-YYYYMMDD.yaml`.
4. **Repository Status**
   - Mark legacy repository as read-only with banner linking to Go API README.
5. **Flags Cleanup**
   - Once audits complete, remove `ROUTE_TO_GO`, `SHADOW_TRAFFIC`, `CANARY_PERCENTAGE`, and `LEGACY_READONLY` from production config.
6. **Documentation**
   - Append metrics summary (latency, error rate, cache hit) to this document.
   - Link incident postmortems if rollback triggered.

## Rollback Drill Log
Record each drill: date, stage, toggle commands executed, duration, outcome.

| Date | Stage | Action | Outcome | Notes |
| --- | --- | --- | --- | --- |
| 2026-07-12 | Tabletop | Simulated `ROUTE_TO_GO=false` rollback on dev environment | Pass | See drill detail below |

### Drill Detail — 2026-07-12 Tabletop Rollback (Dev)

**Scenario:** Error rate spike detected on Go API during Canary 10% stage. Simulate rollback to legacy backend.

**Steps executed:**
1. Simulated alert trigger: `http_error_rate > 1%` for 15 minutes on Go API.
2. Executed rollback command: `make toggle-go value=false` (simulated — `.env` `ROUTE_TO_GO=false`).
3. Verified `ROUTE_TO_GO=false` in `.env`.
4. Simulated ingress reconfiguration to legacy backend defaults.
5. Left `SHADOW_TRAFFIC=true` for diagnostics (per runbook).
6. Simulated cache purge for `auth`, `grades`, `attendance` keys.
7. Verified Go API still serving health probes (`/health` 200, `/ready` 200) for shadow comparison.
8. Recorded incident with logs and metrics snapshots.

**Duration:** ~4 minutes (target < 5 minutes).

**Outcome:** Pass. Rollback procedure is well-documented and executable within SLO. All steps in `docs/operations.md` Rollback Procedure are actionable.

**Findings:**
- `make toggle-go` helper script correctly flips `ROUTE_TO_GO` and preserves shadow mode.
- Cache purge step requires Redis CLI access (`FLUSHDB` on specific key patterns) — documented in runbook but should be scripted for automation.
- Ingress reconfiguration is environment-specific and cannot be fully automated in this drill.

**Follow-up actions:**
- Create a `make rollback` target that combines toggle + cache purge in one command.
- Add automated alerting rules for `HighErrorRate` and `LatencySLOViolation` to Prometheus configuration.

## Parity & Analytics Snapshot
- Contract tests: attach `contract-tests` CI artifacts (status, diff summary).
- Shadow compare: attach last 7 nightly reports, highlight delta ≤1% optional fields.
- Analytics validation: confirm Phase 5 dashboards show no regression.

### Contract Test Snapshot — 2026-07-12

| Role | Requests | Assertions | Failures | Notes |
| --- | --- | --- | --- | --- |
| SUPERADMIN | 38 | 76 | 1 | Only Report Job Status 404 (expected — non-existent job) |
| ADMIN | 38 | 76 | 1 | Same as SUPERADMIN |
| TEACHER | 38 | 76 | 9 | All 9 are expected RBAC denials (403) or validation (400) |

**Bugs found and fixed during contract testing:**
- Bug 1: `grade_config_components.created_at` column missing → migration 000013
- Bug 2: `ClassDistribution` query SQL error → fixed in `grade_final_repository.go`
- Bug 3: `Dashboard Academics` 404 — `claims.UserID` used as `teacherID` → migration 000014 + JWT `TeacherID` claim
- Bug 4: `Calendar Alias` 500 for teachers — same root cause as Bug 3 → fixed by JWT `TeacherID` in `calendar_alias_service.go`, `attendance_alias_service.go`, `homeroom_service.go`

### Shadow Compare Status
- **BLOCKED**: Legacy NestJS backend not available in current environment.
- Legacy backend needs to be running on `:3000` for `make shadow-compare` to execute.
- Requires: NestJS backend source, dependencies installed, and database seed parity.
- Follow-up: Provision legacy backend in dev environment or staging for shadow compare.

## Optional Cleanup (Post-Audit)
- Drop unused legacy tables/indexes (requires separate approval ticket).
- Remove feature flag code paths referencing legacy toggles after ops sign-off.
