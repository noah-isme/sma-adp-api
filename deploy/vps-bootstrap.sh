#!/usr/bin/env bash
set -euo pipefail

# Idempotent Ubuntu 24.04 bootstrap for the single, private 2 GB API VPS.
# Cloudflare's current IP ranges must be supplied by the operator rather than
# copied into a stale repository file.

usage() {
  cat <<'EOF'
Usage: vps-bootstrap.sh --ssh-cidr CIDR --cloudflare-ip-file FILE

The Cloudflare file contains one IPv4/IPv6 CIDR per line. The command must run
as root on Ubuntu 24.04 and refuses to enable UFW without both inputs.

Environment:
  SMA_DEPLOY_USER       deployment account (default: sma-deploy)
  SMA_ROOT_DIR          release root (default: /opt/sma)
EOF
}

ssh_cidr=""
cloudflare_file=""
while (($# > 0)); do
  case "$1" in
    --ssh-cidr)
      [[ $# -ge 2 ]] || { echo "--ssh-cidr requires a value" >&2; exit 2; }
      ssh_cidr="$2"
      shift 2
      ;;
    --cloudflare-ip-file)
      [[ $# -ge 2 ]] || { echo "--cloudflare-ip-file requires a value" >&2; exit 2; }
      cloudflare_file="$2"
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
  echo "run as root" >&2
  exit 1
fi
if [[ -z "$ssh_cidr" || -z "$cloudflare_file" || ! -r "$cloudflare_file" ]]; then
  echo "both --ssh-cidr and a readable --cloudflare-ip-file are required" >&2
  exit 2
fi
if ! command -v apt-get >/dev/null 2>&1 || ! command -v systemctl >/dev/null 2>&1; then
  echo "this bootstrap requires an Ubuntu system with apt and systemd" >&2
  exit 1
fi

deploy_user="${SMA_DEPLOY_USER:-sma-deploy}"
backup_user="${SMA_BACKUP_USER:-sma-backup}"
root_dir="${SMA_ROOT_DIR:-/opt/sma}"
if [[ "$root_dir" == "/" || "$root_dir" == "/etc" || "$root_dir" == "/var" ]]; then
  echo "refusing broad SMA_ROOT_DIR: $root_dir" >&2
  exit 2
fi
if [[ ! "$root_dir" =~ ^/[A-Za-z0-9._/-]+$ ]]; then
  echo "SMA_ROOT_DIR must be an absolute path without whitespace or shell metacharacters" >&2
  exit 2
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl docker.io docker-compose-v2 postgresql-client rclone sudo ufw unattended-upgrades chrony openssl

if ! id "$deploy_user" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "$deploy_user"
fi
if ! id "$backup_user" >/dev/null 2>&1; then
  useradd --system --user-group --home-dir /var/lib/sma-backup --shell /usr/sbin/nologin "$backup_user"
fi

# The GitHub release workflow connects as this dedicated account and uses
# narrowly scoped, passwordless sudo for immutable release operations. Do not
# add it to Docker's root-equivalent group; keep the allow-list explicit.
printf '%s\n' \
  "$deploy_user ALL=(ALL) NOPASSWD: /usr/bin/docker, /usr/local/bin/migrate, /usr/bin/tee, /usr/bin/install, /usr/bin/tar, /usr/bin/cp, /usr/bin/cat, /usr/bin/rm, /usr/bin/test, /usr/bin/chmod, ${root_dir}/releases/*/deploy/*.sh" \
  > /etc/sudoers.d/sma-deploy
chmod 0440 /etc/sudoers.d/sma-deploy
/usr/sbin/visudo -cf /etc/sudoers.d/sma-deploy >/dev/null

install -d -m 0750 -o "$deploy_user" -g "$backup_user" "$root_dir"
install -d -m 0750 -o "$deploy_user" -g "$deploy_user" "$root_dir/releases"
install -d -m 0750 -o root -g "$backup_user" "$root_dir/deploy"
install -d -m 0750 -o root -g "$backup_user" /etc/sma
# The Nginx service runs as UID/GID 101. Keep the private key unreadable to
# other users while granting that container identity access through the bind
# mount; Docker's daemon does not change host-file ownership or permissions.
install -d -m 0750 -o 101 -g 101 /etc/sma/tls
install -d -m 0750 -o root -g "$backup_user" /etc/sma/evidence
install -d -m 0750 -o "$backup_user" -g "$backup_user" /var/backups/sma
chown "$deploy_user:$backup_user" "$root_dir"
chown "$deploy_user:$deploy_user" "$root_dir/releases"
chmod 0750 "$root_dir" "$root_dir/releases" "$root_dir/deploy"

# Keep one gigabyte of swap available on the small host. Never overwrite an
# operator-managed swap file.
if ! swapon --show=NAME --noheadings | grep -q .; then
  if [[ ! -e /swapfile ]]; then
    fallocate -l 1G /swapfile
    chmod 0600 /swapfile
    mkswap /swapfile >/dev/null
  fi
  swapon /swapfile
  grep -qF '/swapfile none swap sw 0 0' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

systemctl enable --now docker chrony unattended-upgrades

# Only Cloudflare may reach the public listeners. SSH is restricted to the
# operator network supplied above; database/cache/observability ports remain
# closed by UFW's default-deny policy.
ufw default deny incoming
ufw default allow outgoing
ufw allow from "$ssh_cidr" to any port 22 proto tcp
while IFS= read -r cloudflare_cidr; do
  [[ -z "$cloudflare_cidr" || "$cloudflare_cidr" == \#* ]] && continue
  ufw allow from "$cloudflare_cidr" to any port 80 proto tcp
  ufw allow from "$cloudflare_cidr" to any port 443 proto tcp
done < "$cloudflare_file"
ufw --force enable

echo "VPS bootstrap complete; install the release bundle under $root_dir/releases"
