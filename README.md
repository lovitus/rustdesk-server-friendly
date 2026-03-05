# rustdesk-server-friendly (Go)

A Go-based CLI for generating RustDesk self-host runbooks with:

- one-line interactive wizard (`rustdesk-friendly`)
- executable local backup command (`rustdesk-friendly apply backup`)
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
bash <(curl -fsSL https://raw.githubusercontent.com/lovitus/rustdesk-server-friendly/main/scripts/install_linux_binary.sh) v1.0.0
```

### Windows PowerShell

```powershell
# latest release (run in current PowerShell session)
iwr -useb https://raw.githubusercontent.com/lovitus/rustdesk-server-friendly/main/scripts/install_windows_binary.ps1 | iex

# specific version
powershell -ExecutionPolicy Bypass -File .\scripts\install_windows_binary.ps1 -Version v1.0.0

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
# interactive wizard (recommended)
rustdesk-friendly

# explicit wizard
rustdesk-friendly wizard --output docs/runbook.md

# non-interactive guide generation
rustdesk-friendly guide --target linux --topic all --host rustdesk.example.com

# migration example: linux -> windows
rustdesk-friendly guide \
  --target cross \
  --topic migrate \
  --migration-source linux \
  --migration-target windows \
  --output docs/linux-to-windows.md

# execute real source backup now (not just generate guide)
rustdesk-friendly apply backup --source windows
```

Important:
- `wizard` and `guide` generate documents only.
- `apply backup` is the command that really creates backup archives.

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
