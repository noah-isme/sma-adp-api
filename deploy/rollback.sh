#!/usr/bin/env bash
set -euo pipefail

# Roll back only to an explicitly supplied, digest-pinned release environment.
# The current release file is copied beside itself before any compose action so
# operators can recover the exact prior input. This script never deletes images,
# volumes, databases, or legacy resources.

usage() {
  cat <<'EOF'
Usage: rollback.sh RELEASE_ENV [--dry-run]

RELEASE_ENV must contain SMA_API_IMAGE, SMA_WORKER_IMAGE, NGINX_IMAGE and the
managed-service settings consumed by docker-compose.production.yml. Images must
use @sha256:<64 hex characters>. Use --dry-run to validate without restarting.

Optional environment:
  COMPOSE_FILE       compose file (default: deploy/docker-compose.production.yml)
  HEALTH_URL         edge readiness URL (default: https://$SERVER_NAME/ready)
  RUN_SMOKE          set true to run make compatibility-smoke after health
EOF
}

release_env="${1:-}"
dry_run=false
if [[ "$release_env" == "--help" || "$release_env" == "-h" ]]; then
  usage
  exit 0
fi
if [[ -z "$release_env" ]]; then
  usage >&2
  exit 2
fi
shift
while (($# > 0)); do
  case "$1" in
    --dry-run) dry_run=true; shift ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

compose_file="${COMPOSE_FILE:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/docker-compose.production.yml}"
[[ -f "$release_env" ]] || { echo "release env not found: $release_env" >&2; exit 2; }
[[ -f "$compose_file" ]] || { echo "compose file not found: $compose_file" >&2; exit 2; }

for key in SMA_API_IMAGE SMA_WORKER_IMAGE NGINX_IMAGE; do
  value="$(awk -F= -v key="$key" '$1 == key {print substr($0, index($0, "=") + 1); exit}' "$release_env")"
  if [[ ! "$value" =~ @sha256:[0-9a-fA-F]{64}$ ]]; then
    echo "$key must be pinned to image@sha256:<64 hex characters>" >&2
    exit 2
  fi
done

compose_dir="$(cd "$(dirname "$compose_file")" && pwd)"
compose_args=(--env-file "$release_env" -f "$compose_file")
if [[ "$dry_run" == true ]]; then
  echo "Validating rollback release without changing deployment state"
  SMA_ENV_FILE="$release_env" docker compose "${compose_args[@]}" config >/dev/null
  echo "Rollback configuration is valid"
  exit 0
fi

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
cp -- "$release_env" "$release_env.rollback.$stamp"

echo "Starting pinned rollback release"
SMA_ENV_FILE="$release_env" docker compose "${compose_args[@]}" up -d --no-deps api nginx

health_url="${HEALTH_URL:-}"
if [[ -z "$health_url" ]]; then
  server_name="$(awk -F= '$1 == "SERVER_NAME" {print substr($0, index($0, "=") + 1); exit}' "$release_env")"
  health_url="https://${server_name}/ready"
fi

for attempt in $(seq 1 30); do
  if curl --fail --silent --show-error --max-time 3 "$health_url" >/dev/null; then
    echo "Rollback readiness check passed: $health_url"
    break
  fi
  if [[ "$attempt" == 30 ]]; then
    echo "Rollback readiness check failed; inspect compose logs and restore the prior release file" >&2
    exit 1
  fi
  sleep 2
done

if [[ "${RUN_SMOKE:-false}" == true ]]; then
  (cd "$compose_dir/.." && make compatibility-smoke)
fi

echo "Rollback completed; previous release input preserved at $release_env.rollback.$stamp"
