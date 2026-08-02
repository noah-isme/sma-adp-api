# Frontend ↔ Backend Mapping

The complete caller → route → handler → flag → casing → envelope → readiness matrix is maintained in [`CONTRACT_MATRIX.md`](CONTRACT_MATRIX.md).

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
- `GET /export/students`, `/export/grades`, `/export/attendance`
- `POST /students/import`, `POST /teachers/import`

Use `Pending` in the matrix until a route has passed backend tests, gateway build, and a smoke request against seeded data.
