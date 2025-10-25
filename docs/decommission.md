# Legacy Decommission Checklist

## Timeline
- **T0:** Full cutover complete, legacy in read-only mode (`LEGACY_READONLY=true`).
- **T0 + 7d:** Confirm SLO stability, complete parity audits.
- **T0 + 14d:** Begin legacy teardown (pipelines, routing, secrets).

## Mandatory Steps
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

## Parity & Analytics Snapshot
- Contract tests: attach `contract-tests` CI artifacts (status, diff summary).
- Shadow compare: attach last 7 nightly reports, highlight delta ≤1% optional fields.
- Analytics validation: confirm Phase 5 dashboards show no regression.

## Optional Cleanup (Post-Audit)
- Drop unused legacy tables/indexes (requires separate approval ticket).
- Remove feature flag code paths referencing legacy toggles after ops sign-off.
