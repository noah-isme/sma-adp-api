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
coverage is verified. Keep seeded runtime status separate until a live database-backed
request has been executed; the optional harness is `scripts/compatibility_smoke.py`.
