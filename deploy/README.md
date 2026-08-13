# Production deployment bundle

This directory describes the production topology and contains the immutable
runtime contract:

`Cloudflare (WAF + Full Strict TLS) → Nginx (VPS) → Go API → managed PostgreSQL/Redis`

The default 2 GB VPS footprint contains only `api` and `nginx`. The report queue
is currently in-process, so the `worker` Compose profile is intentionally
disabled until a dedicated worker entrypoint is delivered. Prometheus,
Alertmanager, and Grafana remain available through the explicit
`observability` profile, but lightweight VPS checks are the default.

## One-time VPS preparation

Bootstrap an Ubuntu 24.04 VPS with the current Cloudflare IP ranges and the
operator SSH network. The script is idempotent and refuses an unscoped
firewall. It installs Docker Compose v2, PostgreSQL client tools used by the
backup verifier, and rclone for the encrypted backup remote:

```bash
sudo deploy/vps-bootstrap.sh \
  --ssh-cidr 203.0.113.10/32 \
  --cloudflare-ip-file /root/cloudflare-origin-ranges.txt
sudo install -m 0640 deploy/env.production.example /etc/sma/sma-api.env
# Edit /etc/sma/sma-api.env using the secret manager. Never commit it.
sudo install -o 101 -g 101 -m 0640 fullchain.pem /etc/sma/tls/fullchain.pem
sudo install -o 101 -g 101 -m 0640 privkey.pem /etc/sma/tls/privkey.pem
```

If the GHCR package is private, configure a read-only package token in the
root Docker credential store before the first release (the Compose command is
run through `sudo`):

```bash
printf '%s' "$GHCR_READ_TOKEN" | sudo docker login ghcr.io \
  --username "$GHCR_READ_USER" --password-stdin
```

Keep this token outside the repository and rotate it independently from the
GitHub Actions token that publishes images.

The bootstrap creates the default `sma-deploy` account, creates the
`sma-backup` service account, and installs an auditable
passwordless-sudo allow-list for release operations. Use `sma-deploy` for
`SMA_VPS_USER` in the GitHub production environment; do not use root SSH for
releases. Install the backup scripts under `/opt/sma/deploy` with group
`sma-backup` as described in `backup-runbook.md`.

Install the pinned `golang-migrate` CLI as `/usr/local/bin/migrate` before the
first release; the approval-gated workflow invokes it on the VPS. Keep Go and
the CLI available for an operator-led restore drill, which runs the migration
integrity checker against an isolated database.

The release file must replace every `REPLACE_WITH_64_HEX_DIGEST` with the
digest returned by the registry. `DB_SSL_MODE=require` and `REDIS_TLS=true`
are mandatory; use managed endpoints, private networking, and credentials
with only the permissions needed by this API. Permit inbound 80/443 only from
Cloudflare and SSH only from the operations network.

## Release build and deploy

Build the API for the VPS architecture, push one immutable image, and record
the registry digest before changing the VPS:

```bash
cd sma-adp-api
export RELEASE=v$(git rev-parse --short=12 HEAD)
docker buildx build --platform linux/amd64 \
  --file deploy/Dockerfile.api \
  --tag ghcr.io/ORG/sma-api:${RELEASE} --push .
docker buildx imagetools inspect ghcr.io/ORG/sma-api:${RELEASE}

python3 deploy/validate.py
docker compose --env-file /etc/sma/sma-api.env \
  -f deploy/docker-compose.production.yml config >/tmp/sma-compose-${RELEASE}.yml
```

Copy the checked release bundle to the VPS, then validate and start the edge
and API. The default command does not start the worker or observability
profiles:

```bash
sudo cp /etc/sma/sma-api.env /etc/sma/sma-api.env.previous
sudo docker compose --env-file /etc/sma/sma-api.env \
  -f /opt/sma/deploy/docker-compose.production.yml config >/dev/null
sudo docker compose --env-file /etc/sma/sma-api.env \
  -f /opt/sma/deploy/docker-compose.production.yml \
  up -d api nginx
curl --fail --show-error https://api.example.com/health
curl --fail --show-error https://api.example.com/ready
```

`api` uses a distroless non-root runtime, a read-only root
filesystem, dropped capabilities, no-new-privileges, and explicit CPU/memory
limits sized for a 2 GB host. Nginx listens on container ports 8080/8443 so it
can run non-root while the VPS maps ports 80/443. Nginx routes directly to Go;
there is no legacy upstream or request mirroring. `/metrics`, `/docs`,
`/internal/*`, `/debug/*`, and `/api/v1/portal/*` return 404 publicly.

To opt into the heavier dashboards during an incident or a later capacity
review, use `--profile observability` and supply the pinned image digests:

```bash
sudo docker compose --env-file /etc/sma/sma-api.env \
  -f /opt/sma/deploy/docker-compose.production.yml \
  --profile observability up -d
```

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

## Lightweight monitoring

Install `monitor.sh` as a root-owned systemd timer. It checks API/nginx
containers, public readiness, available memory, backup age, filesystem space,
and TLS certificate expiry; alert on a non-zero exit status. The default
thresholds are appropriate for a 2 GB VPS and can be overridden with the
documented `SMA_*` variables.

```bash
sudo install -m 0750 deploy/monitor.sh /opt/sma/deploy/monitor.sh
sudo /opt/sma/deploy/monitor.sh
```

Keep the existing Prometheus/Grafana configuration for an explicitly enabled
profile or a later monitoring host. It is not required for the first launch.

## Evidence record template

Copy this block into the release record and attach command output:

```text
Release commit/digest:
Operator + UTC window:
Environment/hostname:
API image@sha256:
Nginx image@sha256:
Migration version + integrity JSON:
Backup checksum + restore target/evidence:
Cloudflare Full Strict/TLS/WAF/rate-limit evidence:
Health/readiness output:
Rollback drill timestamp and recovery duration:
VPS monitor evidence:
Approvals:
```
