#!/usr/bin/env bash
set -euo pipefail

# Lightweight VPS monitor for the default api + nginx deployment. This is
# intentionally independent of Prometheus/Grafana so a 2 GB host still has a
# useful liveness signal. Exit status 1 is suitable for a systemd timer or a
# cron/mail wrapper.

usage() {
  cat <<'EOF'
Usage: monitor.sh [--help]

Environment:
  SMA_COMPOSE_FILE        compose file (default: deploy/docker-compose.production.yml)
  SMA_ENV_FILE            release env file (default: /etc/sma/sma-api.env)
  SMA_HEALTH_URL          public health URL (default: https://SERVER_NAME/ready)
  SMA_BACKUP_DIR          backup directory (default: /var/backups/sma)
  SMA_TLS_DIR             origin TLS directory (default: /etc/sma/tls)
  SMA_MIN_DISK_PERCENT    minimum free disk percentage (default: 15)
  SMA_MIN_MEMORY_MB       minimum available memory (default: 256)
  SMA_CERT_MIN_DAYS       minimum certificate lifetime (default: 14)
  SMA_BACKUP_MAX_AGE_HOURS maximum backup age (default: 26)
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi
if (($# > 0)); then
  echo "unknown option: $1" >&2
  usage >&2
  exit 2
fi

compose_file="${SMA_COMPOSE_FILE:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/docker-compose.production.yml}"
env_file="${SMA_ENV_FILE:-/etc/sma/sma-api.env}"
backup_dir="${SMA_BACKUP_DIR:-/var/backups/sma}"
tls_dir="${SMA_TLS_DIR:-/etc/sma/tls}"
min_disk="${SMA_MIN_DISK_PERCENT:-15}"
min_memory="${SMA_MIN_MEMORY_MB:-256}"
cert_min_days="${SMA_CERT_MIN_DAYS:-14}"
backup_max_age="${SMA_BACKUP_MAX_AGE_HOURS:-26}"

[[ -f "$compose_file" ]] || { echo "compose file not found: $compose_file" >&2; exit 1; }
[[ -f "$env_file" ]] || { echo "release env not found: $env_file" >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }
command -v df >/dev/null 2>&1 || { echo "df is required" >&2; exit 1; }
command -v free >/dev/null 2>&1 || { echo "free is required" >&2; exit 1; }

server_name="$(awk -F= '$1 == "SERVER_NAME" {print substr($0, index($0, "=") + 1); exit}' "$env_file")"
health_url="${SMA_HEALTH_URL:-https://${server_name}/ready}"

failures=0
check() {
  local label="$1"
  shift
  if "$@"; then
    echo "PASS $label"
  else
    echo "FAIL $label" >&2
    failures=$((failures + 1))
  fi
}

compose=(docker compose --env-file "$env_file" -f "$compose_file")
running_services="$("${compose[@]}" ps --status running --services 2>/dev/null || true)"
check "api container running" grep -Eq '(^|\n)api(\n|$)' <<<"$running_services"
check "nginx container running" grep -Eq '(^|\n)nginx(\n|$)' <<<"$running_services"
check "public readiness" curl --fail --silent --show-error --max-time 10 "$health_url"

if [[ -d "$backup_dir" ]]; then
  disk_free="$(df -P "$backup_dir" | awk 'NR == 2 {gsub(/%/, "", $5); print 100 - $5}')"
  check "backup filesystem has ${min_disk}% free" test "${disk_free:-0}" -ge "$min_disk"
else
  echo "FAIL backup directory missing: $backup_dir" >&2
  failures=$((failures + 1))
fi

available_memory="$(free -m | awk '/^Mem:/ {print $7}')"
check "at least ${min_memory} MiB memory available" test "${available_memory:-0}" -ge "$min_memory"

for certificate in "$tls_dir/fullchain.pem" "$tls_dir/privkey.pem"; do
  if [[ ! -r "$certificate" ]]; then
    echo "FAIL TLS file unreadable: $certificate" >&2
    failures=$((failures + 1))
  fi
done
if [[ -r "$tls_dir/fullchain.pem" ]] && command -v openssl >/dev/null 2>&1; then
  check "TLS certificate valid for ${cert_min_days} days" \
    openssl x509 -in "$tls_dir/fullchain.pem" -noout -checkend "$((cert_min_days * 86400))"
fi

latest_backup="$(find "$backup_dir" -type f -name '*.dump' -printf '%T@\n' 2>/dev/null | sort -nr | head -1 || true)"
if [[ -n "$latest_backup" ]]; then
  backup_age="$(awk -v now="$(date +%s)" -v stamp="$latest_backup" 'BEGIN {print int((now - stamp) / 3600)}')"
  check "backup is newer than ${backup_max_age} hours" test "$backup_age" -le "$backup_max_age"
else
  echo "FAIL no PostgreSQL dump found under $backup_dir" >&2
  failures=$((failures + 1))
fi

if ((failures > 0)); then
  echo "VPS monitor failed with ${failures} check(s)" >&2
  exit 1
fi
echo "VPS monitor passed"
