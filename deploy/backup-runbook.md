# PostgreSQL backup and restore runbook

`backup.sh` is the scheduled, provider-neutral wrapper around
`verify-backup.sh`. It creates a custom-format dump, verifies the catalogue and
checksum, writes JSON evidence, and uploads those artifacts to an operator-
configured `rclone` `crypt` remote. It never deletes local or remote files;
retention is owned by the storage provider's lifecycle policy. The default VPS
monitor fails if no successful dump is present within 26 hours.

## Configure the encrypted remote

Create a dedicated least-privilege rclone remote whose backend type is
`crypt`, backed by the organization's object storage. Keep the rclone config
outside the repository and make the remote readable only by the backup service
account. Set:

```text
DATABASE_URL=postgres://backup-user:...@managed-postgres:5432/admin_panel_sma?sslmode=require
SMA_BACKUP_DIR=/var/backups/sma
SMA_BACKUP_REMOTE=sma-crypt:production
SMA_BACKUP_REMOTE_PATH=database
SMA_BACKUP_ENCRYPTED=true
```

The wrapper fails closed if `SMA_BACKUP_ENCRYPTED=true` is absent or the named
rclone remote is not `type = crypt`.

## systemd schedule

Install `backup.sh` and `verify-backup.sh` under `/opt/sma/deploy/`, owned by
`root:sma-backup` with mode `0750`. The bootstrap creates the `sma-backup`
service account, the encrypted-backup directory, and the `rclone` package.
Store the environment in `/etc/sma/backup.env` with mode `0640` and group
`sma-backup`; do not reuse `/etc/sma/sma-api.env`, which contains API secrets.

`/etc/systemd/system/sma-backup.service`:

```ini
[Unit]
Description=SMA PostgreSQL encrypted backup
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=sma-backup
Group=sma-backup
EnvironmentFile=/etc/sma/backup.env
ExecStart=/opt/sma/deploy/backup.sh
```

`/etc/systemd/system/sma-backup.timer`:

```ini
[Unit]
Description=Daily SMA PostgreSQL backup

[Timer]
OnCalendar=*-*-* 02:00:00 UTC
Persistent=true
RandomizedDelaySec=15m

[Install]
WantedBy=timers.target
```

Enable it with `systemctl daemon-reload && systemctl enable --now
sma-backup.timer`. Alert on a failed unit and on the absence of a successful
run within 26 hours; `Persistent=true` covers a host that was offline at the
scheduled time.

## Restore drill

Run `verify-backup.sh --restore-url` against a separately named
`restore`/`staging`/`drill` database with `RESTORE_CONFIRM=I_UNDERSTAND`. Never
use the production database as the restore target. Record the generated
`restore-integrity.json`, the release commit, elapsed restore time, and the
operator sign-off as release evidence. Download artifacts through the rclone
remote into an isolated directory before beginning a drill.
