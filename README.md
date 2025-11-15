# SMA ADP API (Golang)

Backend inisialisasi untuk migrasi dari NestJS ke Golang (Phase 0). Jalankan dev env via Docker.

## Quickstart
```bash
cp .env.example .env
make docker-up
make dev
```

## Docs
- Swagger: `/docs` (dev only)
- Health: `/health`, `/ready`
- Internal health diff: `/internal/ping-legacy`, `/internal/ping-go`
- Cutover runbook: [`docs/operations.md`](docs/operations.md)
- Decommission checklist: [`docs/decommission.md`](docs/decommission.md)
- FE ↔ BE mapping: [`docs/FE_BE_MAPPING.md`](docs/FE_BE_MAPPING.md)

## Makefile
Lihat target via `make help`.
