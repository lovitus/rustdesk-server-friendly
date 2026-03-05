# rustdesk-server-friendly

A practical helper app for RustDesk self-hosting with:

- one-line interactive CLI wizard (no parameter memorization)
- auto-detect paths from running service/PM2/NSSM config
- multi-channel pull steps (`wget`/`curl` on Linux, `Invoke-WebRequest`/`curl.exe`/`gh+winget` on Windows)
- Linux and Windows command generation
- desktop GUI for non-CLI operators
- log size and retention limits (avoid disk-full incidents)
- service installation instructions (systemd / PM2 / NSSM)
- guided migration checklists and commands for all pairs:
  - Linux -> Linux
  - Linux -> Windows
  - Windows -> Linux
  - Windows -> Windows

## Why this project

Self-hosting RustDesk is easy to start but often hard to standardize:

- people do not know which binaries to install
- service setup differs by OS
- logs may fill the disk if not rotated
- migration is risky without a clear file checklist

This tool makes those steps repeatable and exportable.

## Installation

Prerequisites:

- Python 3.9+
- Git

Option A (recommended, isolated CLI install with `pipx`):

```bash
git clone https://github.com/lovitus/rustdesk-server-friendly.git
cd rustdesk-server-friendly
pipx install .
```

Option B (virtual environment):

```bash
git clone https://github.com/lovitus/rustdesk-server-friendly.git
cd rustdesk-server-friendly
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
```

Windows PowerShell (`venv` example):

```powershell
git clone https://github.com/lovitus/rustdesk-server-friendly.git
cd rustdesk-server-friendly
py -3 -m venv .venv
.venv\Scripts\Activate.ps1
python -m pip install -e .
```

If you do not want to install command entrypoints, you can run directly:

```bash
python -m rustdesk_server_friendly
```

## Quick start

```bash
# one-line interactive wizard (recommended)
rustdesk-friendly

# explicit wizard command
rustdesk-friendly wizard

# direct non-interactive generation (optional)
rustdesk-friendly guide --target linux --topic all --host rustdesk.example.com
```

## CLI modes

1. `wizard` (recommended)
- asks questions interactively
- exports runbook to markdown
- no need to remember parameters
- defaults to auto-detecting source/target data directories from running services

2. `guide`
- deterministic output for automation/CI
- supports explicit migration pair selection
- supports pinned version and download integrity env vars in generated scripts (`RUSTDESK_RELEASE_TAG`, `RUSTDESK_ZIP_SHA256`)

Examples:

```bash
# Migration Linux -> Windows
rustdesk-friendly guide \
  --target cross \
  --topic migrate \
  --migration-source linux \
  --migration-target windows \
  --source-linux-data-dir /var/lib/rustdesk-server \
  --target-windows-dir "C:\\RustDesk-Server" \
  --output docs/linux-to-windows.md

# Migration Windows -> Windows
rustdesk-friendly guide \
  --target cross \
  --topic migrate \
  --migration-source windows \
  --migration-target windows \
  --source-windows-dir "C:\\Old-RustDesk" \
  --target-windows-dir "D:\\RustDesk-Server" \
  --output docs/windows-to-windows.md
```

## Idempotent behavior

Generated scripts include explicit guards to avoid accidental overwrite:

- `[SKIP]` when binaries/configs/services already exist
- `[STOP]` when partial/conflicting state is detected
- `FORCE_REINSTALL=1` and `ALLOW_OVERWRITE=1` as explicit opt-in switches
- `RUSTDESK_SOURCE_DATA_DIR` / `RUSTDESK_TARGET_DATA_DIR` for manual override when auto-detection is wrong
- post-apply checks (service state + port listening tests) for operator confidence

## GUI mode

```bash
rustdesk-friendly gui
```

If your Python build has no `tkinter` (common on minimal Linux/macOS builds), CLI still works and GUI will print a clear dependency message.

## Verification

```bash
pytest -q
```

## Sources used for command design

- [RustDesk OSS server repository](https://github.com/rustdesk/rustdesk-server)
- [RustDesk Windows self-host docs](https://rustdesk.com/docs/en/self-host/rustdesk-server-oss/windows/)
- [RustDesk Pro convert-from-OSS script](https://raw.githubusercontent.com/rustdesk/rustdesk-server-pro/main/convertfromos.sh)

## License

MIT
