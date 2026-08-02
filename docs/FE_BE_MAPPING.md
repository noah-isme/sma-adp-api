# Frontend ↔ Backend Mapping

The compatibility caller → route → handler → flag → casing → envelope → readiness matrix is maintained in [`COMPATIBILITY_CONTRACT_MATRIX.md`](COMPATIBILITY_CONTRACT_MATRIX.md). It is not an exhaustive core-resource CRUD inventory; use Swagger and the API specification for that surface.

This file is intentionally a navigation page, not a second source of truth. Compatibility routes covered by the matrix include:

- `/exam-events`
- `PUT /enrollments/:id`
- `POST /attendance`
- `PUT/PATCH /attendance/:id`
- `POST /teacher-preferences`
- `PUT /teacher-preferences/:id`
- `GET /students/roster`
- `GET /teachers/roster`
- `GET /grades/report`
- `PATCH /grades/:id`
- `PUT /grades/:id` (generic admin provider compatibility)
- `DELETE /grades/:id` (soft delete and final-grade recalculation)
- `PUT /grade-components/:id`
- `DELETE /grade-components/:id` (soft delete)
- `GET /export/students`, `/export/grades`, `/export/attendance`
- `POST /students/import`, `POST /teachers/import` (5 MiB/10,000-row limits, idempotency replay, row-level failures, and audit records)

The matrix records `Static contract smoke passed` when gateway and Swagger method/path
coverage is verified. The latest seeded runtime verification (Postgres/Redis,
migrations 15–18, seed data, and compatibility flags) passed on 2026-08-02; its
coverage and the remaining browser-only row are recorded per route in the matrix.
The repeatable read-only harness is `scripts/compatibility_smoke.py`; current Go
package tests remain blocked by the environment's module-cache/disk quota (see
[`PROJECT_STATUS.md`](PROJECT_STATUS.md)).
