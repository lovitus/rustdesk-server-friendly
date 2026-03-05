# rustdesk-server-friendly (Go)

A Go-based CLI for generating RustDesk self-host runbooks with:

- one-run interactive workflow (`rustdesk-friendly`)
- executable local backup command (`rustdesk-friendly apply backup`)
- executable import/restore command (`rustdesk-friendly apply import`)
- idempotent deploy/service/log scripts (`[SKIP]` / `[STOP]` guards)
- migration guides for all pairs:
  - Linux -> Linux
  - Linux -> Windows
  - Windows -> Linux
  - Windows -> Windows
- auto-detection hints for runtime data paths (service/PM2/NSSM)
- multiple download methods:
  - Linux: `wget` / `curl`
  - Windows: `Invoke-WebRequest` / `curl.exe` / `gh` (install via `winget`)

## Install (Prebuilt Binary)

### Linux

```bash
# latest release, auto choose curl/wget
bash <(curl -fsSL https://raw.githubusercontent.com/lovitus/rustdesk-server-friendly/main/scripts/install_linux_binary.sh)

# or pin a release tag
bash <(curl -fsSL https://raw.githubusercontent.com/lovitus/rustdesk-server-friendly/main/scripts/install_linux_binary.sh) v1.0.1
```

### Windows PowerShell

```powershell
# latest release (run in current PowerShell session)
iwr -useb https://raw.githubusercontent.com/lovitus/rustdesk-server-friendly/main/scripts/install_windows_binary.ps1 | iex

# specific version
powershell -ExecutionPolicy Bypass -File .\scripts\install_windows_binary.ps1 -Version v1.0.1

# after install
rustdesk-friendly
```

Notes:
- Do not run raw paths with spaces without call syntax.
- If you must run full path, use:
  - `& "C:\Users\<You>\AppData\Local\rustdesk-server-friendly\rustdesk-friendly.exe"`
- The installer adds `%LOCALAPPDATA%\rustdesk-server-friendly\bin` to user PATH.

## Build From Source

Prerequisites:

- Go 1.26+
- Git

```bash
git clone https://github.com/lovitus/rustdesk-server-friendly.git
cd rustdesk-server-friendly
go build -o rustdesk-friendly ./cmd/rustdesk-friendly
./rustdesk-friendly version
```

Windows:

```powershell
git clone https://github.com/lovitus/rustdesk-server-friendly.git
cd rustdesk-server-friendly
go build -o rustdesk-friendly.exe .\cmd\rustdesk-friendly
.\rustdesk-friendly.exe version
```

## Usage

```bash
# interactive app (recommended)
rustdesk-friendly

# execute real source backup
rustdesk-friendly apply backup --source windows

# execute import/restore from archive
rustdesk-friendly apply import --target linux --archive /tmp/rustdesk-migration-backup.tgz
```

Important:
- `backup` is read-only: no service stop, no source file modification, no deletion.
- `import` validates archive structure before writing anything.
- interactive mode provides 3 choices: `backup` / `import` / `generate-guide`.

## Quality Controls in Generated Scripts

- install conflict protection: `FORCE_REINSTALL=1`
- migration overwrite protection: `ALLOW_OVERWRITE=1`
- optional release pin: `RUSTDESK_RELEASE_TAG`
- optional archive hash check: `RUSTDESK_ZIP_SHA256`
- path override fallback: `RUSTDESK_SOURCE_DATA_DIR`, `RUSTDESK_TARGET_DATA_DIR`

## Test

```bash
go test ./...
```

## Release Assets

Published binaries follow these names:

- `rustdesk-friendly-linux-amd64`
- `rustdesk-friendly-linux-arm64`
- `rustdesk-friendly-windows-amd64.exe`
- `rustdesk-friendly-darwin-amd64`
- `rustdesk-friendly-darwin-arm64`

## Sources

- https://github.com/rustdesk/rustdesk-server
- https://rustdesk.com/docs/en/self-host/rustdesk-server-oss/windows/
- https://raw.githubusercontent.com/rustdesk/rustdesk-server-pro/main/convertfromos.sh

## License

MIT
