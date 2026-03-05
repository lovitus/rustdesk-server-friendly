# rustdesk-server-friendly

A practical helper app for RustDesk self-hosting with:

- Linux and Windows command generation
- Desktop GUI for non-CLI operators
- Log size and retention limits (avoid disk-full incidents)
- Service installation instructions (systemd / PM2 / NSSM)
- Windows Server -> Linux Server migration checklist and commands

## Why this project

Self-hosting RustDesk is easy to start but often hard to standardize:

- people do not know which binaries to install
- service setup differs by OS
- logs may fill the disk if not rotated
- migration from Windows to Linux is risky without a clear file checklist

This tool makes those steps repeatable and exportable.

## Features

1. CLI mode
- Generate markdown guides directly in terminal or files.
- Example:

```bash
python -m rustdesk_server_friendly guide --target linux --topic all --host my-rustdesk.example.com
```

2. GUI mode
- Desktop UI for selecting target/topic and copying scripts.
- Example:

```bash
python -m rustdesk_server_friendly gui
```

3. Topics
- `deploy`: binary download + initial setup
- `service`: install/enable service instances
- `logs`: log rotation + retention policy
- `migrate`: Windows -> Linux migration
- `all`: all available topics for target

4. Targets
- `linux`
- `windows`
- `cross` (for migration topic)

## Quick start

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -e .

# CLI output in terminal
rustdesk-friendly guide --target windows --topic service --host 203.0.113.10

# Export to file
rustdesk-friendly guide --target cross --topic migrate --output docs/windows-to-linux-migration.md

# Launch GUI
rustdesk-friendly gui
```

If your Python build has no `tkinter` (common on minimal Linux/macOS builds), CLI still works and GUI will print a clear dependency message.

## Example output

Generate a full Linux playbook:

```bash
rustdesk-friendly guide --target linux --topic all --host rustdesk.your-domain.com --output linux-runbook.md
```

## Migration essentials covered by this app

When moving from Windows Server to Linux Server, this app highlights the critical files:

- `id_ed25519`
- `id_ed25519.pub`
- `db_v2.sqlite3` (+ `-wal` / `-shm` when present)

It also includes stop/backup/restore/verify commands and a cutover checklist.

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
