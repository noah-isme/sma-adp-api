#!/usr/bin/env bash
set -euo pipefail

# Create a PostgreSQL custom-format backup and verify that it can be listed.
# With --restore-url, restore into an explicitly isolated database and rerun
# the read-only migration integrity checks. The restore guard intentionally
# rejects URLs that do not name a restore/staging/drill database.

usage() {
  cat <<'EOF'
Usage: verify-backup.sh [options]

Options:
  --source-url URL     source PostgreSQL URL (default: DATABASE_URL)
  --backup-dir DIR     destination directory (default: temporary directory)
  --restore-url URL    isolated restore target; requires RESTORE_CONFIRM
  --keep               keep temporary backup/evidence files
  --help               show this help

Environment:
  DATABASE_URL         source URL when --source-url is omitted
  RESTORE_CONFIRM      must equal I_UNDERSTAND for an isolated restore
EOF
}

source_url="${DATABASE_URL:-}"
backup_dir=""
restore_url=""
keep=false

while (($# > 0)); do
  case "$1" in
    --source-url)
      [[ $# -ge 2 ]] || { echo "--source-url requires a value" >&2; exit 2; }
      source_url="$2"
      shift 2
      ;;
    --backup-dir)
      [[ $# -ge 2 ]] || { echo "--backup-dir requires a value" >&2; exit 2; }
      backup_dir="$2"
      shift 2
      ;;
    --restore-url)
      [[ $# -ge 2 ]] || { echo "--restore-url requires a value" >&2; exit 2; }
      restore_url="$2"
      shift 2
      ;;
    --keep)
      keep=true
      shift
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

if [[ -z "$source_url" ]]; then
  echo "source URL is required; set DATABASE_URL or pass --source-url" >&2
  exit 2
fi
if ! command -v pg_dump >/dev/null 2>&1 || ! command -v pg_restore >/dev/null 2>&1; then
  echo "pg_dump and pg_restore are required" >&2
  exit 2
fi

if [[ -n "$backup_dir" ]]; then
  case "$backup_dir" in
    /|""|/etc|/var|/usr|/home)
      echo "refusing unsafe backup directory: $backup_dir" >&2
      exit 2
      ;;
  esac
  mkdir -p "$backup_dir"
  work_dir="$backup_dir"
  cleanup_work=false
else
  work_dir="$(mktemp -d "${TMPDIR:-/tmp}/sma-backup.XXXXXX")"
  cleanup_work=true
fi

cleanup() {
  if [[ "$cleanup_work" == true && "$keep" != true ]]; then
    rm -rf -- "$work_dir"
  fi
}
trap cleanup EXIT

backup_file="$work_dir/sma-$(date -u +%Y%m%dT%H%M%SZ).dump"
evidence_file="$work_dir/restore-integrity.json"

echo "Creating PostgreSQL custom-format backup: $backup_file"
pg_dump --format=custom --no-owner --no-privileges --file "$backup_file" "$source_url"
[[ -s "$backup_file" ]] || { echo "backup is empty" >&2; exit 1; }

echo "Verifying backup catalogue"
pg_restore --list "$backup_file" >/dev/null
sha256sum "$backup_file" | tee "$backup_file.sha256"

if [[ -n "$restore_url" ]]; then
  if [[ "${RESTORE_CONFIRM:-}" != "I_UNDERSTAND" ]]; then
    echo "RESTORE_CONFIRM=I_UNDERSTAND is required for an isolated restore" >&2
    exit 2
  fi
  if [[ "$source_url" == "$restore_url" ]]; then
    echo "refusing to restore onto the source database" >&2
    exit 2
  fi
  if [[ ! "$restore_url" =~ (restore|staging|drill) ]]; then
    echo "restore target must identify an isolated restore/staging/drill database" >&2
    exit 2
  fi

  echo "Restoring into isolated target"
  pg_restore --exit-on-error --clean --if-exists --no-owner --no-privileges \
    --dbname "$restore_url" "$backup_file"

  repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  echo "Running read-only migration integrity checks"
  (
    cd "$repo_root"
    go run ./scripts/migration_integrity --dsn "$restore_url" --json > "$evidence_file"
  )
  echo "Restore evidence: $evidence_file"
fi

echo "Backup verification passed"
