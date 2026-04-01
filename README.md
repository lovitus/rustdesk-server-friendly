# rustdesk-server-friendly

Go-based RustDesk lifecycle manager.

Default experience:

- download
- run
- choose one action:
  - `new-service`
  - `backup-migrate`
  - `restore-service`
  - `diagnose-repair`

The program now favors execution over runbook generation:

- detects runtime, service manager, directories, binaries, and common conflicts
- keeps backup read-only on the source side
- creates a structured backup package with manifest and restore plan
- validates archive restorable state before declaring backup success
- restores through staging and rollback-aware flow
- supports isolated live-restore verification and archive verification marking
- downloads upstream RustDesk Server binaries when the target host does not already have them
- executes Linux `systemd` and Windows service registration when the local runtime and permissions allow it
- prefers Windows `NSSM`, then `pm2`, then native `sc` when registering services
- requires triple confirmation for `new-service` and `restore-service` when existing RustDesk service/data is detected

## Support Matrix

- Linux: `amd64`, `arm64`, `armv7`
- Windows: `amd64`, `arm64`
- macOS: `amd64`, `arm64`

macOS is limited to backup, archive validation, restore planning, and isolated local verification. It does not provide managed service hosting.

## Install

### Linux

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/lovitus/rustdesk-server-friendly/main/scripts/install_linux_binary.sh)
rustdesk-friendly
```

### Windows PowerShell

```powershell
iwr -useb https://raw.githubusercontent.com/lovitus/rustdesk-server-friendly/main/scripts/install_windows_binary.ps1 | iex
rustdesk-friendly
```

## Main Flows

### Interactive

```bash
rustdesk-friendly
```

### New Service

```bash
rustdesk-friendly new-service
```

If the machine already contains RustDesk service/data/ports, the program blocks until the operator completes all three confirmations.

### Backup / Migrate

```bash
rustdesk-friendly apply backup --source linux --output /tmp/rustdesk-lifecycle-backup.tgz
```

Backup rules:

- never stops the source service
- never edits source files
- never deletes source files
- packs data, detected binaries, detected service definitions, log snapshot metadata, and `manifest.json`
- reopens the archive and verifies required restore content before returning success
- in interactive mode, backup can immediately continue into isolated live-restore verification on the same host
- writes backup reports beside the archive as `rustdesk-friendly-backup-report.json` and `.md`

Verification levels:

- `archive_valid`
- `restorable_verified`
- `live_restore_verified`

### Restore Service

```bash
rustdesk-friendly apply import --target linux --archive /tmp/rustdesk-lifecycle-backup.tgz --force --triple-confirmed
```

Useful flags:

- `--validate-only`
- `--live-verify`
- `--user-confirmed-live`
- `--triple-confirmed`

Interactive backup now also offers:

- immediate isolated restore verification on the current host
- service creation for `-verify` instances
- operator confirmation to promote the archive from `restorable_verified` to `live_restore_verified`

Restore behavior:

- validates manifest and required files
- prepares staging extraction
- creates rollback copy for conflicting target files
- restores into target or isolated verification directory
- auto-downloads target binaries when they are missing
- registers managed services on Linux/Windows when supported by the runtime and current permissions
- performs post-restore health checks
- can mark the archive as `live_restore_verified` after operator confirmation
- writes verification reports into the target or isolated verification directory

### Diagnose / Repair

```bash
rustdesk-friendly diagnose
```

This prints runtime support, detected service manager, detected data directory, and common port conflicts.

## Advanced Mode

The generated guide flow still exists, but it is now an advanced path:

```bash
rustdesk-friendly guide --target linux --topic all
```

## Build From Source

Prerequisites:

- Go `1.26+`
- Git

```bash
git clone https://github.com/lovitus/rustdesk-server-friendly.git
cd rustdesk-server-friendly
go build -o rustdesk-friendly ./cmd/rustdesk-friendly
./rustdesk-friendly version
```

## Test

```bash
go test ./...
```

## Notes

- Linux service management targets `systemd`.
- Windows service management prefers `NSSM`, then `pm2`, then native `sc`.
- Cross-platform restore uses source metadata plus target runtime mapping and can fetch matching upstream binaries for the destination runtime.
- Windows installer auto-selects `amd64` or `arm64` release assets based on the local architecture.
- Test and CI flows can disable real download or service execution with:
  - `RUSTDESK_FRIENDLY_SKIP_DOWNLOAD=1`
  - `RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL=1`
  - `RUSTDESK_FRIENDLY_SKIP_SC=1`

## Sources

- https://github.com/rustdesk/rustdesk-server
- https://rustdesk.com/docs/en/self-host/rustdesk-server-oss/windows/
- https://raw.githubusercontent.com/rustdesk/rustdesk-server-pro/main/convertfromos.sh

## License

MIT
