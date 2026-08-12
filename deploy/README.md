# Production deployment bundle

This directory describes the production topology and contains the immutable
runtime contract:

`Cloudflare (WAF + Full Strict TLS) → Nginx (VPS) → Go API → managed PostgreSQL/Redis`

The API and worker images are separate release inputs. The report queue is
currently in-process, so the `worker` Compose profile is intentionally disabled
until a dedicated worker entrypoint is delivered. Do not enable that profile in
production merely to increase capacity; it would start a second API process.

## One-time VPS preparation

Run as an operator with Docker Compose v2, `curl`, `pg_dump`, `pg_restore`, Go,
and the `migrate` CLI installed:

```bash
sudo install -d -m 0750 /etc/sma/tls /etc/sma/evidence /opt/sma
sudo install -m 0640 deploy/env.production.example /etc/sma/sma-api.env
# Edit /etc/sma/sma-api.env using the secret manager. Never commit it.
sudo install -m 0640 fullchain.pem /etc/sma/tls/fullchain.pem
sudo install -m 0600 privkey.pem /etc/sma/tls/privkey.pem
```

The release file must replace every `REPLACE_WITH_64_HEX_DIGEST` with the
digest returned by the registry. `DB_SSL_MODE=require` and `REDIS_TLS=true`
are mandatory; use managed endpoints, private networking, and credentials
with only the permissions needed by this API.

## Release build and deploy

Build for the VPS architecture, push both immutable images, and record the
registry digests before changing the VPS:

```bash
cd sma-adp-api
export RELEASE=v$(git rev-parse --short=12 HEAD)
docker buildx build --platform linux/amd64 \
  --file deploy/Dockerfile.api \
  --tag ghcr.io/ORG/sma-api:${RELEASE} --push .
docker buildx build --platform linux/amd64 \
  --file deploy/Dockerfile.worker \
  --tag ghcr.io/ORG/sma-worker:${RELEASE} --push .
docker buildx imagetools inspect ghcr.io/ORG/sma-api:${RELEASE}
docker buildx imagetools inspect ghcr.io/ORG/sma-worker:${RELEASE}

python3 deploy/validate.py
docker compose --env-file /etc/sma/sma-api.env \
  -f deploy/docker-compose.production.yml config >/tmp/sma-compose-${RELEASE}.yml
```

Copy the checked release file to the VPS, then validate and start the edge,
API, and observability services. The command is safe to repeat and does not
start the disabled worker profile:

```bash
sudo cp /etc/sma/sma-api.env /etc/sma/sma-api.env.previous
sudo docker compose --env-file /etc/sma/sma-api.env \
  -f /opt/sma/deploy/docker-compose.production.yml config >/dev/null
sudo docker compose --env-file /etc/sma/sma-api.env \
  -f /opt/sma/deploy/docker-compose.production.yml \
  up -d api nginx prometheus alertmanager grafana
curl --fail --show-error https://api.example.com/health
curl --fail --show-error https://api.example.com/ready
```

`api` and `worker` use a distroless non-root runtime, a read-only root
filesystem, dropped capabilities, no-new-privileges, and explicit CPU/memory
limits. Nginx listens on container ports 8080/8443 so it can run non-root while
the VPS maps ports 80/443. Prometheus reaches `/metrics` only over the internal
Docker network; Nginx returns 404 for that endpoint and `/internal/*`.

## Migration integrity evidence

Apply migrations using the production `DB_URL` secret, then run the read-only
validator. It runs every check in a PostgreSQL `READ ONLY` transaction and exits
1 if any orphan or grade-calculation discrepancy is found:

```bash
export DB_URL='postgres://...managed.../admin_panel_sma?sslmode=require'
migrate -path migrations -database "$DB_URL" up
go run ./scripts/migration_integrity --dsn "$DB_URL" --json \
  | tee /etc/sma/evidence/migration-integrity-$(date -u +%Y%m%dT%H%M%SZ).json
```

Attach the JSON, migration version, release digest, and operator/timezone to
the release record. A non-zero result blocks traffic promotion.

## Backup and restore verification

The helper creates a custom-format dump, verifies its catalogue, records a
SHA-256 checksum, and optionally restores into an explicitly isolated database.
The restore target must contain `restore`, `staging`, or `drill` in its database
name and requires an explicit confirmation string:

```bash
export DATABASE_URL='postgres://...managed.../admin_panel_sma?sslmode=require'
deploy/verify-backup.sh --backup-dir /var/backups/sma --keep

export RESTORE_CONFIRM=I_UNDERSTAND
deploy/verify-backup.sh \
  --source-url "$DATABASE_URL" \
  --restore-url 'postgres://.../admin_panel_sma_restore_20260812?sslmode=require' \
  --backup-dir /var/backups/sma/restore-drill --keep
```

Never pass the production URL as `--restore-url`. Preserve the `.dump`, `.sha256`,
and `restore-integrity.json` as the restore-test evidence.

## Controlled cutover

Nginx hashes the Cloudflare client address to keep a client on one upstream and
uses the existing cutover flags from the deployment secret. It never mirrors
state-changing requests. For each stage, update the secret, render the config,
and reload only after the health and parity checks pass:

```bash
# Shadow comparison is read-only and remains outside the Nginx write path.
# Run from a staging runner that can resolve both private upstream hostnames.
make shadow-compare GO_BASE_URL=https://go.internal.example.com LEGACY_BASE_URL=https://legacy.internal.example.com

# Canary stages: keep legacy as the default until each hold period passes.
ROUTE_TO_GO=true CANARY_PERCENTAGE=10
ROUTE_TO_GO=true CANARY_PERCENTAGE=50
ROUTE_TO_GO=true CANARY_PERCENTAGE=100 LEGACY_READONLY=true

sudo docker compose --env-file /etc/sma/sma-api.env \
  -f /opt/sma/deploy/docker-compose.production.yml up -d --no-deps nginx
curl --fail https://api.example.com/ready
make decommission-preflight PREFLIGHT_ARGS="--env-file /etc/sma/sma-api.env"
```

Hold 10% for 24 hours, 50% for 48 hours, and 100% for 24 hours. Promotion
requires error rate ≤0.5%, p95 ≤250 ms, schema parity ≥99%, and no integrity
regression. Alert or manually rollback at error rate >1% or p99 >600 ms for
15 minutes, or immediately for security/data-integrity regressions.

## Rollback

Rollback accepts only a release environment containing digest-pinned images. A
dry run validates the Compose expansion without changing state:

```bash
deploy/rollback.sh /etc/sma/releases/sma-api.previous.env --dry-run
sudo HEALTH_URL=https://api.example.com/ready \
  deploy/rollback.sh /etc/sma/releases/sma-api.previous.env
```

The script copies the supplied release input beside itself before restarting
`api` and `nginx`, waits up to 60 seconds for readiness, and can run the
compatibility smoke with `RUN_SMOKE=true`. It does not remove images/volumes or
flush Redis; preserve the prior image and backup for the recovery window.

## Monitoring

The Compose stack mounts the existing alert rules and Grafana dashboard from
the repository. Before production, replace the Alertmanager SMTP placeholders
using the secret-management process and route `operations-email` to the staffed
operations mailbox. Validate artifacts from the repository root:

```bash
cd ..
python3 monitoring/validate.py
promtool check rules monitoring/prometheus/sma-api-alerts.yml
```

Exercise `HighErrorRate`, `LatencySLOViolation`, `CacheMissSpike`, and
`DBSlowQuery` in staging and save Alertmanager delivery screenshots/logs.
Grafana is bound to `127.0.0.1:3001`; expose it only through VPN or an
authenticated tunnel, never by opening it to the public internet.

## Evidence record template

Copy this block into the release record and attach command output:

```text
Release commit/digest:
Operator + UTC window:
Environment/hostname:
API image@sha256:
Worker image@sha256 (not enabled while queue is in-process):
Nginx/monitoring image digests:
Migration version + integrity JSON:
Backup checksum + restore target/evidence:
Cloudflare Full Strict/TLS/WAF/rate-limit evidence:
Health/readiness output:
Shadow schema parity:
10% hold (start/end, error%, p95/p99):
50% hold (start/end, error%, p95/p99):
100% hold (start/end, error%, p95/p99):
Rollback drill timestamp and recovery duration:
Alert exercise delivery evidence:
Approvals:
```
