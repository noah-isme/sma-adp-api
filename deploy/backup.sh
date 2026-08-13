#!/usr/bin/env bash
set -euo pipefail

# Run the local backup verifier and copy its immutable evidence to an
# operator-configured encrypted rclone remote. Retention and deletion stay
# with the remote provider's lifecycle policy; this script never deletes a
# local or remote backup.

usage() {
  cat <<'EOF'
Usage: backup.sh [options]

Options:
  --source-url URL     source PostgreSQL URL (default: DATABASE_URL)
  --backup-dir DIR     local backup root (default: SMA_BACKUP_DIR or /var/backups/sma)
  --remote REMOTE      encrypted rclone remote (default: SMA_BACKUP_REMOTE)
  --remote-path PATH   remote prefix (default: SMA_BACKUP_REMOTE_PATH or sma)
  --restore-url URL    isolated restore target passed to verify-backup.sh
  --help               show this help

Environment:
  DATABASE_URL              source URL when --source-url is omitted
  SMA_BACKUP_DIR             local backup root
  SMA_BACKUP_REMOTE         rclone remote, for example sma-crypt:production
  SMA_BACKUP_REMOTE_PATH    remote prefix below the configured remote
  SMA_BACKUP_ENCRYPTED      must equal true; the remote must be an encrypted rclone remote
  RESTORE_CONFIRM           must equal I_UNDERSTAND for an isolated restore

The command uploads the dump, its SHA-256 checksum, and JSON evidence. It
fails closed on a missing/non-crypt rclone remote or any upload error.
EOF
}

source_url="${DATABASE_URL:-}"
backup_root="${SMA_BACKUP_DIR:-/var/backups/sma}"
remote="${SMA_BACKUP_REMOTE:-}"
remote_path="${SMA_BACKUP_REMOTE_PATH:-sma}"
restore_url=""

while (($# > 0)); do
  case "$1" in
    --source-url)
      [[ $# -ge 2 ]] || { echo "--source-url requires a value" >&2; exit 2; }
      source_url="$2"
      shift 2
      ;;
    --backup-dir)
      [[ $# -ge 2 ]] || { echo "--backup-dir requires a value" >&2; exit 2; }
      backup_root="$2"
      shift 2
      ;;
    --remote)
      [[ $# -ge 2 ]] || { echo "--remote requires a value" >&2; exit 2; }
      remote="$2"
      shift 2
      ;;
    --remote-path)
      [[ $# -ge 2 ]] || { echo "--remote-path requires a value" >&2; exit 2; }
      remote_path="$2"
      shift 2
      ;;
    --restore-url)
      [[ $# -ge 2 ]] || { echo "--restore-url requires a value" >&2; exit 2; }
      restore_url="$2"
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

if [[ -z "$source_url" ]]; then
  echo "source URL is required; set DATABASE_URL or pass --source-url" >&2
  exit 2
fi
if [[ -z "$remote" ]]; then
  echo "encrypted rclone remote is required; set SMA_BACKUP_REMOTE or pass --remote" >&2
  exit 2
fi
if [[ "${SMA_BACKUP_ENCRYPTED:-}" != "true" ]]; then
  echo "SMA_BACKUP_ENCRYPTED=true is required for offsite backup" >&2
  exit 2
fi
if [[ "$remote" != *:* ]]; then
  echo "rclone remote must include a remote name and colon" >&2
  exit 2
fi
if [[ "$backup_root" =~ [^a-zA-Z0-9_./-] ]]; then
  echo "backup directory contains unsafe characters: $backup_root" >&2
  exit 2
fi
if [[ "$remote_path" =~ [^a-zA-Z0-9_./-] ]]; then
  echo "remote path contains unsafe characters: $remote_path" >&2
  exit 2
fi

command -v rclone >/dev/null 2>&1 || { echo "rclone is required" >&2; exit 2; }

remote_name="${remote%%:*}"
remote_config="$(rclone config show "$remote_name" 2>/dev/null || true)"
if ! grep -Eq '^type[[:space:]]*=[[:space:]]*crypt$' <<<"$remote_config"; then
  echo "rclone remote '$remote_name' must use the crypt backend" >&2
  exit 2
fi

run_id="$(date -u +%Y%m%dT%H%M%SZ)"
work_dir="${backup_root%/}/$run_id"
mkdir -p "$work_dir"

verify_args=(--source-url "$source_url" --backup-dir "$work_dir" --keep)
if [[ -n "$restore_url" ]]; then
  verify_args+=(--restore-url "$restore_url")
fi
"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/verify-backup.sh" "${verify_args[@]}"

shopt -s nullglob
dump_files=("$work_dir"/*.dump)
shopt -u nullglob
if [[ "${#dump_files[@]}" -ne 1 ]]; then
  echo "expected exactly one PostgreSQL dump in $work_dir" >&2
  exit 1
fi
dump_file="${dump_files[0]}"
checksum_file="$dump_file.sha256"
[[ -s "$checksum_file" ]] || { echo "missing backup checksum: $checksum_file" >&2; exit 1; }
sha256sum --check "$checksum_file" --status

digest="$(awk '{print $1}' "$checksum_file")"
size_bytes="$(wc -c < "$dump_file" | tr -d '[:space:]')"
evidence_file="$work_dir/backup-evidence.json"
created_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
dump_name="$(basename "$dump_file")"
printf '{"created_at":"%s","backup_file":"%s","sha256":"%s","size_bytes":%s}\n' \
  "$created_at" "$dump_name" "$digest" "$size_bytes" >"$evidence_file"

remote_target="${remote%/}/${remote_path%/}/$run_id"
for artifact in "$dump_file" "$checksum_file" "$evidence_file"; do
  echo "Uploading $(basename "$artifact") to $remote_target"
  rclone copyto --checksum "$artifact" "$remote_target/$(basename "$artifact")"
done

shopt -s nullglob
restore_evidence=("$work_dir"/restore-integrity.json)
shopt -u nullglob
for artifact in "${restore_evidence[@]}"; do
  [[ -f "$artifact" ]] || continue
  echo "Uploading $(basename "$artifact") to $remote_target"
  rclone copyto --checksum "$artifact" "$remote_target/$(basename "$artifact")"
done

echo "Encrypted backup upload passed: $remote_target"
