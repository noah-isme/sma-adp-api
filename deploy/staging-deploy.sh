#!/usr/bin/env bash
set -euo pipefail

# Promote one immutable API image to the isolated staging VPS.
#
# This script is intentionally executed through the deploy account's narrowly
# scoped sudo rule. It applies migrations only after a PostgreSQL backup, runs
# the read-only integrity checker before changing the running image, and keeps
# the last known-good release environment for a digest-pinned rollback.

usage() {
  cat <<'EOF'
Usage: staging-deploy.sh --release-dir DIR --env-file FILE
  --api-image IMAGE@sha256:DIGEST --health-url URL [options]

Required:
  --release-dir DIR       extracted release bundle directory
  --env-file FILE         existing staging runtime env file (outside the repo)
  --api-image IMAGE       API image reference with a 64-character digest
  --health-url URL        HTTPS edge readiness URL

Options:
  --project-name NAME     Compose project name (default: sma-staging)
  --current-env FILE      current release pointer (derived from release dir)
  --previous-env FILE     previous release pointer (derived from release dir)
  --backup-dir DIR        backup root (default: /var/backups/sma)
  --migrate-bin FILE      golang-migrate binary (default: /usr/local/bin/migrate)
  --help                  show this help

The staging runtime env must contain the managed PostgreSQL and Redis settings
used by docker-compose.production.yml. DATABASE_URL or DB_URL is preferred for
migrations; DB_* settings are accepted as a fallback.
EOF
}

release_dir=""
runtime_env=""
api_image=""
health_url=""
project_name="sma-staging"
current_env=""
previous_env=""
backup_root="${SMA_STAGING_BACKUP_DIR:-/var/backups/sma}"
migrate_bin="${MIGRATE_BIN:-/usr/local/bin/migrate}"

while (($# > 0)); do
  case "$1" in
    --release-dir)
      [[ $# -ge 2 ]] || { echo "--release-dir requires a value" >&2; exit 2; }
      release_dir="$2"
      shift 2
      ;;
    --env-file)
      [[ $# -ge 2 ]] || { echo "--env-file requires a value" >&2; exit 2; }
      runtime_env="$2"
      shift 2
      ;;
    --api-image)
      [[ $# -ge 2 ]] || { echo "--api-image requires a value" >&2; exit 2; }
      api_image="$2"
      shift 2
      ;;
    --health-url)
      [[ $# -ge 2 ]] || { echo "--health-url requires a value" >&2; exit 2; }
      health_url="$2"
      shift 2
      ;;
    --project-name)
      [[ $# -ge 2 ]] || { echo "--project-name requires a value" >&2; exit 2; }
      project_name="$2"
      shift 2
      ;;
    --current-env)
      [[ $# -ge 2 ]] || { echo "--current-env requires a value" >&2; exit 2; }
      current_env="$2"
      shift 2
      ;;
    --previous-env)
      [[ $# -ge 2 ]] || { echo "--previous-env requires a value" >&2; exit 2; }
      previous_env="$2"
      shift 2
      ;;
    --backup-dir)
      [[ $# -ge 2 ]] || { echo "--backup-dir requires a value" >&2; exit 2; }
      backup_root="$2"
      shift 2
      ;;
    --migrate-bin)
      [[ $# -ge 2 ]] || { echo "--migrate-bin requires a value" >&2; exit 2; }
      migrate_bin="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "${EUID}" -ne 0 ]]; then
  echo "staging-deploy.sh must run as root (invoke it through sudo)" >&2
  exit 1
fi

for name in release_dir runtime_env api_image health_url; do
  if [[ -z "${!name}" ]]; then
    echo "--${name//_/-} is required" >&2
    exit 2
  fi
done

valid_absolute_path() {
  [[ "$1" =~ ^/[A-Za-z0-9._/-]+$ && "$1" != "/" ]]
}

for path_value in "$release_dir" "$runtime_env" "$backup_root"; do
  if ! valid_absolute_path "$path_value"; then
    echo "unsafe absolute path: $path_value" >&2
    exit 2
  fi
done

if [[ -z "$current_env" ]]; then
  current_env="$(dirname "$release_dir")/staging.current.env"
fi
if [[ -z "$previous_env" ]]; then
  previous_env="$(dirname "$release_dir")/staging.previous.env"
fi
for path_value in "$current_env" "$previous_env" "$migrate_bin"; do
  if ! valid_absolute_path "$path_value"; then
    echo "unsafe absolute path: $path_value" >&2
    exit 2
  fi
done

if [[ ! "$api_image" =~ @sha256:[0-9a-fA-F]{64}$ ]]; then
  echo "--api-image must use image@sha256:<64 hex characters>" >&2
  exit 2
fi
if [[ ! "$health_url" =~ ^https://[^[:space:]]+/ready/?$ ]]; then
  echo "--health-url must be an HTTPS URL ending in /ready" >&2
  exit 2
fi
if [[ ! "$project_name" =~ ^[a-z0-9][a-z0-9_-]{0,62}$ ]]; then
  echo "--project-name contains unsafe characters" >&2
  exit 2
fi

compose_file="$release_dir/deploy/docker-compose.production.yml"
integrity_bin="$release_dir/migration-integrity"
backup_script="$release_dir/deploy/verify-backup.sh"
rollback_script="$release_dir/deploy/rollback.sh"
release_env="$release_dir/release.env"
integrity_report="$release_dir/migration-integrity.json"
backup_evidence="$release_dir/backup-evidence.json"
migrations_dir="$release_dir/migrations"

for required_file in "$runtime_env" "$compose_file" "$integrity_bin" "$backup_script" "$rollback_script"; do
  [[ -f "$required_file" ]] || { echo "required release file not found: $required_file" >&2; exit 1; }
done
[[ -d "$migrations_dir" ]] || { echo "migrations directory not found: $migrations_dir" >&2; exit 1; }
[[ -x "$integrity_bin" ]] || { echo "migration integrity binary is not executable: $integrity_bin" >&2; exit 1; }
[[ -x "$migrate_bin" ]] || { echo "migrate binary is not executable: $migrate_bin" >&2; exit 1; }

# Copy the operator-owned environment without printing it. The release file is
# the exact input needed by Compose and rollback, while SMA_ENV_FILE still
# points at the original secret-bearing runtime file.
awk -v image="$api_image" -v env_file="$runtime_env" '
  BEGIN { image_seen = 0; env_file_seen = 0 }
  /^SMA_API_IMAGE=/ { print "SMA_API_IMAGE=" image; image_seen = 1; next }
  /^SMA_ENV_FILE=/ { print "SMA_ENV_FILE=" env_file; env_file_seen = 1; next }
  { print }
  END {
    if (!image_seen) print "SMA_API_IMAGE=" image
    if (!env_file_seen) print "SMA_ENV_FILE=" env_file
  }
' "$runtime_env" > "$release_env"
chmod 0640 "$release_env"

env_value() {
  awk -F= -v key="$1" '$1 == key {print substr($0, index($0, "=") + 1); exit}' "$release_env"
}

# Every service in the checked-in Compose contract has an image input, even
# when its profile is disabled. Requiring all of them here prevents a staging
# release from silently falling back to a floating tag or placeholder.
for image_key in SMA_API_IMAGE SMA_WORKER_IMAGE NGINX_IMAGE PROMETHEUS_IMAGE ALERTMANAGER_IMAGE GRAFANA_IMAGE; do
  image_value="$(env_value "$image_key")"
  if [[ ! "$image_value" =~ @sha256:[0-9a-fA-F]{64}$ ]]; then
    echo "$image_key must be pinned to image@sha256:<64 hex characters> in $runtime_env" >&2
    exit 2
  fi
done

# Load only the trusted, root-owned operator env. A URL supplied explicitly is
# preferred because it preserves URL-escaped passwords and TLS options.
unset DATABASE_URL DB_URL
set -a
# shellcheck disable=SC1090
. "$runtime_env"
set +a
database_url="${DATABASE_URL:-${DB_URL:-}}"
if [[ -z "$database_url" ]]; then
  : "${DB_HOST:?DB_HOST or DATABASE_URL/DB_URL is required in staging env}"
  : "${DB_PORT:?DB_PORT or DATABASE_URL/DB_URL is required in staging env}"
  : "${DB_USER:?DB_USER or DATABASE_URL/DB_URL is required in staging env}"
  : "${DB_NAME:?DB_NAME or DATABASE_URL/DB_URL is required in staging env}"
  database_url="postgres://${DB_USER}:${DB_PASSWORD:-}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE:-require}"
fi

if [[ -f "$current_env" ]]; then
  cp -- "$current_env" "$previous_env"
  chmod 0640 "$previous_env"
fi

release_id="$(basename "$release_dir")"
backup_dir="${backup_root%/}/$release_id"
install -d -m 0750 "$backup_dir"

echo "Creating staging database backup"
"$backup_script" --source-url "$database_url" --backup-dir "$backup_dir" --keep

echo "Applying database migrations"
"$migrate_bin" -path "$migrations_dir" -database "$database_url" up
migration_version="$("$migrate_bin" -path "$migrations_dir" -database "$database_url" version 2>&1)"
echo "Migration version: $migration_version"

echo "Running read-only migration integrity checks"
"$integrity_bin" --dsn "$database_url" --json > "$integrity_report"
chmod 0640 "$integrity_report"
if ! grep -q '"passed": true' "$integrity_report"; then
  echo "migration integrity checks failed; current application release was not changed" >&2
  exit 1
fi

compose=(docker compose --project-name "$project_name" --env-file "$release_env" -f "$compose_file")
"${compose[@]}" config >/dev/null

echo "Pulling digest-pinned staging images"
"${compose[@]}" pull api nginx

echo "Starting staging API and edge"
"${compose[@]}" up -d api nginx

ready=false
for attempt in $(seq 1 30); do
  if curl --fail --silent --show-error --max-time 5 "$health_url" >/dev/null; then
    ready=true
    break
  fi
  sleep 2
done

if [[ "$ready" != true ]]; then
  echo "staging readiness failed; collecting container state before rollback" >&2
  "${compose[@]}" ps >&2 || true
  "${compose[@]}" logs --tail=100 api nginx >&2 || true
  if [[ -f "$previous_env" ]]; then
    echo "Rolling back to the previous digest-pinned release" >&2
    COMPOSE_PROJECT_NAME="$project_name" "$rollback_script" "$previous_env" --health-url "$health_url"
  else
    echo "No previous staging release exists; manual recovery is required" >&2
  fi
  exit 1
fi

# Keep a small, non-secret evidence file beside the release. The dump and its
# checksum remain in the dedicated backup directory and are never exposed in
# CI logs.
backup_file="$(find "$backup_dir" -maxdepth 1 -type f -name '*.dump' -print -quit)"
checksum_file="${backup_file}.sha256"
[[ -s "$backup_file" && -s "$checksum_file" ]] || { echo "backup evidence files are missing" >&2; exit 1; }
backup_sha256="$(awk '{print $1}' "$checksum_file")"
printf '{"release":"%s","backup_file":"%s","backup_sha256":"%s","migration_version":"%s"}\n' \
  "$release_id" "$(basename "$backup_file")" "$backup_sha256" "$migration_version" > "$backup_evidence"
chmod 0640 "$backup_evidence"

cp -- "$release_env" "$current_env"
chmod 0640 "$current_env"
echo "Staging deployment passed: release=$release_id image=$api_image"
