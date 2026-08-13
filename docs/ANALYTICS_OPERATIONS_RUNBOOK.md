# Analytics and production operations runbook

This runbook is the operational companion to the analytics API contract. It
covers the Phase 5 drilldowns and leaderboards plus the Phase 6 controls that
must be enabled before a production cutover.

## Analytics contract

Set `ENABLE_ANALYTICS=true` only after the analytics migrations have been
applied. The authenticated endpoints are:

| Endpoint | Required query | Optional query |
| --- | --- | --- |
| `/api/v1/analytics/class/{class_id}` | `term_id` | — |
| `/api/v1/analytics/student/{student_id}` | `term_id` | — |
| `/api/v1/analytics/subject/{subject_id}` | `term_id` | `class_id` |
| `/api/v1/analytics/leaderboard/{gpa,attendance,behavior}` | `term_id` | `class_id`, `limit` |

`limit` defaults to 10 and must be between 1 and 100. Request parameters and
response fields are `snake_case`; successful responses use the standard
`{data, pagination, meta}` envelope. Administrators can inspect all records,
teachers are limited to assigned classes/subjects, and a student can inspect
only the student identifier in their JWT. Missing resource/term tuples return
404; malformed query values return 400.

## Materialized-view lifecycle

Migration `000021_advanced_analytics_mvs` is retained as a compatibility
no-op for databases that already recorded the earlier migration. Migration
`000026_correct_analytics_mvs` owns the corrected, pre-aggregated views used by
the repository. Apply migrations in order from an empty database and verify
that the views exist before enabling the feature flag:

```sql
SELECT to_regclass('mv_class_statistics'),
       to_regclass('mv_student_performance'),
       to_regclass('mv_subject_statistics');
SELECT refresh_analytics_mvs();
```

Refresh the views after a material grade, attendance, or behavior import. The
service cache is intentionally scoped by endpoint, term, class, subject, and
student filters; invalidate or wait for the configured TTL after a refresh.

## Production controls

- The Go token bucket is a defense-in-depth fallback. Keep the Cloudflare and
  Nginx limits enabled, set Nginx `limit_req_status 429`, and use the canonical
  client IP when the service is behind a trusted proxy.
- Apply the security-header middleware globally and verify HSTS, CSP,
  `X-Content-Type-Options`, `X-Frame-Options`, and `Referrer-Policy` from the
  public endpoint.
- Schedule `deploy/backup.sh` with the encrypted `rclone` remote described in
  `deploy/backup-runbook.md`. Alert when a successful backup is older than 26
  hours and perform restore drills against an isolated database.
- Set `PORTAL_PASSWORD_RESET_URL` to the deployed portal origin. Startup
  validation rejects the localhost default in production.

## Release verification

Run the Go suite, frontend tests/builds, deployment validators, and Swagger
route validation before enabling analytics. `make contract-test` additionally
requires an `ACCESS_TOKEN`; record its authenticated result and the staging
portal E2E report in the release checklist. A local passing run does not replace
staging acceptance or a documented backup restore drill.
