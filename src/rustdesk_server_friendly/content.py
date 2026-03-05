"""Guide content generator for RustDesk self-host operations."""

from __future__ import annotations

from textwrap import dedent

SUPPORTED_TARGETS = ("linux", "windows", "cross")
SUPPORTED_TOPICS = ("deploy", "logs", "service", "migrate", "all")

SOURCES = {
    "rustdesk_server_readme": "https://github.com/rustdesk/rustdesk-server",
    "rustdesk_windows_doc": "https://rustdesk.com/docs/en/self-host/rustdesk-server-oss/windows/",
    "rustdesk_pro_convert_script": "https://raw.githubusercontent.com/rustdesk/rustdesk-server-pro/main/convertfromos.sh",
}


def render_guide(
    target: str,
    topic: str,
    host: str = "<PUBLIC_HOST_OR_IP>",
    windows_dir: str = r"C:\RustDesk-Server",
    linux_install_dir: str = "/opt/rustdesk-server",
    linux_data_dir: str = "/var/lib/rustdesk-server",
    linux_log_dir: str = "/var/log/rustdesk-server",
) -> str:
    """Render markdown guidance for a specific target/topic pair."""
    target = target.lower().strip()
    topic = topic.lower().strip()

    if target not in SUPPORTED_TARGETS:
        raise ValueError(f"Unsupported target: {target}")
    if topic not in SUPPORTED_TOPICS:
        raise ValueError(f"Unsupported topic: {topic}")

    host = host.strip() or "<PUBLIC_HOST_OR_IP>"

    sections: list[str] = []
    sections.append(_title(target, topic))

    if target == "linux":
        if topic in ("deploy", "all"):
            sections.append(_linux_deploy(host, linux_install_dir, linux_data_dir, linux_log_dir))
        if topic in ("service", "all"):
            sections.append(_linux_service(host, linux_install_dir, linux_data_dir, linux_log_dir))
        if topic in ("logs", "all"):
            sections.append(_linux_log_limits(linux_log_dir))
        if topic == "migrate":
            sections.append(
                "`migrate` topic is cross-platform. Use `--target cross --topic migrate` instead."
            )

    if target == "windows":
        if topic in ("deploy", "all"):
            sections.append(_windows_deploy(host, windows_dir))
        if topic in ("service", "all"):
            sections.append(_windows_service(host, windows_dir))
        if topic in ("logs", "all"):
            sections.append(_windows_log_limits())
        if topic == "migrate":
            sections.append(
                "`migrate` topic is cross-platform. Use `--target cross --topic migrate` instead."
            )

    if target == "cross":
        if topic in ("migrate", "all"):
            sections.append(_windows_to_linux_migration(windows_dir, linux_data_dir))
        else:
            sections.append("For `cross` target, only `migrate` or `all` is valid.")

    sections.append(_source_notes())
    return "\n\n".join(sections).strip() + "\n"


def _title(target: str, topic: str) -> str:
    return dedent(
        f"""
        # RustDesk Server Friendly Guide

        - Target: `{target}`
        - Topic: `{topic}`
        """
    ).strip()


def _linux_deploy(host: str, install_dir: str, data_dir: str, log_dir: str) -> str:
    return dedent(
        f"""
        ## Linux CLI Deploy (Binary)

        ```bash
        set -euo pipefail

        # 1) Base packages
        sudo apt-get update
        sudo apt-get install -y curl tar unzip

        # 2) Prepare folders
        sudo install -d -m 0755 {install_dir}/bin {data_dir} {log_dir}

        # 3) Fetch latest RustDesk Server OSS release
        TAG=$(curl -fsSL https://api.github.com/repos/rustdesk/rustdesk-server/releases/latest | awk -F '"' '/tag_name/{{print $4; exit}}')
        ARCH=$(uname -m)
        case "$ARCH" in
          x86_64) PKG_ARCH="amd64" ;;
          aarch64|arm64) PKG_ARCH="arm64" ;;
          armv7l) PKG_ARCH="armv7" ;;
          *) echo "Unsupported arch: $ARCH"; exit 1 ;;
        esac

        TMPDIR=$(mktemp -d)
        trap 'rm -rf "$TMPDIR"' EXIT

        curl -fL "https://github.com/rustdesk/rustdesk-server/releases/download/${{TAG}}/rustdesk-server-linux-${{PKG_ARCH}}.tar.gz" -o "$TMPDIR/rustdesk-server.tar.gz"
        tar -xzf "$TMPDIR/rustdesk-server.tar.gz" -C "$TMPDIR"

        HBBS=$(find "$TMPDIR" -type f -name hbbs | head -n1)
        HBBR=$(find "$TMPDIR" -type f -name hbbr | head -n1)
        [ -n "$HBBS" ] && [ -n "$HBBR" ]

        sudo install -m 0755 "$HBBS" {install_dir}/bin/hbbs
        sudo install -m 0755 "$HBBR" {install_dir}/bin/hbbr

        # 4) Open required ports
        # TCP: 21115-21117, 21118; UDP: 21116
        sudo ufw allow 21115:21118/tcp || true
        sudo ufw allow 21116/udp || true

        echo "Binary deploy prepared. Next: install services."
        ```

        GUI-friendly option:
        - Run this app with `--gui`, pick `linux + deploy`, then copy/export the generated commands.
        """
    ).strip()


def _linux_service(host: str, install_dir: str, data_dir: str, log_dir: str) -> str:
    return dedent(
        f"""
        ## Linux Service Install (systemd)

        ```bash
        set -euo pipefail

        sudo tee /etc/systemd/system/rustdesk-hbbs.service >/dev/null <<'UNIT'
        [Unit]
        Description=RustDesk HBBS
        After=network.target

        [Service]
        Type=simple
        WorkingDirectory={data_dir}
        ExecStart={install_dir}/bin/hbbs -r {host}:21117
        Restart=always
        RestartSec=5
        LimitNOFILE=1048576
        StandardOutput=append:{log_dir}/hbbs.log
        StandardError=append:{log_dir}/hbbs.error.log

        [Install]
        WantedBy=multi-user.target
        UNIT

        sudo tee /etc/systemd/system/rustdesk-hbbr.service >/dev/null <<'UNIT'
        [Unit]
        Description=RustDesk HBBR
        After=network.target

        [Service]
        Type=simple
        WorkingDirectory={data_dir}
        ExecStart={install_dir}/bin/hbbr
        Restart=always
        RestartSec=5
        LimitNOFILE=1048576
        StandardOutput=append:{log_dir}/hbbr.log
        StandardError=append:{log_dir}/hbbr.error.log

        [Install]
        WantedBy=multi-user.target
        UNIT

        sudo systemctl daemon-reload
        sudo systemctl enable --now rustdesk-hbbs rustdesk-hbbr
        sudo systemctl status rustdesk-hbbs --no-pager
        sudo systemctl status rustdesk-hbbr --no-pager
        ```
        """
    ).strip()


def _linux_log_limits(log_dir: str) -> str:
    return dedent(
        f"""
        ## Linux Log Limits

        ```bash
        set -euo pipefail

        # 1) Rotate RustDesk file logs
        sudo tee /etc/logrotate.d/rustdesk-server >/dev/null <<'CONF'
        {log_dir}/*.log {{
            daily
            rotate 14
            size 50M
            missingok
            notifempty
            compress
            delaycompress
            copytruncate
        }}
        CONF

        # 2) Limit journald usage to avoid disk exhaustion
        sudo install -d /etc/systemd/journald.conf.d
        sudo tee /etc/systemd/journald.conf.d/rustdesk.conf >/dev/null <<'CONF'
        [Journal]
        SystemMaxUse=500M
        RuntimeMaxUse=200M
        MaxRetentionSec=14day
        CONF

        sudo systemctl restart systemd-journald
        sudo logrotate -f /etc/logrotate.d/rustdesk-server
        ```
        """
    ).strip()


def _windows_deploy(host: str, windows_dir: str) -> str:
    return dedent(
        f"""
        ## Windows CLI Deploy (PowerShell)

        ```powershell
        $ErrorActionPreference = "Stop"

        $Root = "{windows_dir}"
        $Bin = Join-Path $Root "bin"
        $Data = Join-Path $Root "data"
        $Logs = Join-Path $Root "logs"

        New-Item -ItemType Directory -Force -Path $Bin, $Data, $Logs | Out-Null

        $Tag = (Invoke-RestMethod "https://api.github.com/repos/rustdesk/rustdesk-server/releases/latest").tag_name
        $ZipPath = Join-Path $env:TEMP "rustdesk-server.zip"

        Invoke-WebRequest -Uri "https://github.com/rustdesk/rustdesk-server/releases/download/$Tag/rustdesk-server-windows-x64.zip" -OutFile $ZipPath
        Expand-Archive -Path $ZipPath -DestinationPath $Bin -Force

        # Move binaries up if they are in a nested folder
        $hbbs = Get-ChildItem -Path $Bin -Filter hbbs.exe -Recurse | Select-Object -First 1
        $hbbr = Get-ChildItem -Path $Bin -Filter hbbr.exe -Recurse | Select-Object -First 1
        Copy-Item $hbbs.FullName (Join-Path $Bin "hbbs.exe") -Force
        Copy-Item $hbbr.FullName (Join-Path $Bin "hbbr.exe") -Force

        Write-Host "Binary deploy prepared. Next: install service manager (PM2 or NSSM)."
        ```

        GUI-friendly option:
        - Run this app with `--gui`, pick `windows + deploy`, then copy/export commands.
        """
    ).strip()


def _windows_service(host: str, windows_dir: str) -> str:
    return dedent(
        f"""
        ## Windows Service Install (PM2)

        ```powershell
        $ErrorActionPreference = "Stop"

        # 1) Install Node.js LTS and PM2 stack
        winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements
        npm install -g pm2 pm2-windows-startup pm2-logrotate
        pm2-startup install

        # 2) Start RustDesk processes under PM2
        $Bin = Join-Path "{windows_dir}" "bin"
        $Data = Join-Path "{windows_dir}" "data"

        Set-Location $Bin
        pm2 start .\\hbbs.exe --name rustdesk-hbbs -- -r {host}:21117
        pm2 start .\\hbbr.exe --name rustdesk-hbbr

        # 3) Persist startup
        pm2 save
        ```

        Alternative GUI-style service install (NSSM):
        - Run `nssm install` to open the GUI wizard and register `hbbs.exe` / `hbbr.exe`.
        - In NSSM I/O tab, point stdout/stderr to `{windows_dir}\\logs\\*.log`.
        """
    ).strip()


def _windows_log_limits() -> str:
    return dedent(
        """
        ## Windows Log Limits

        ```powershell
        # PM2 log rotation (recommended with PM2 service mode)
        pm2 install pm2-logrotate
        pm2 set pm2-logrotate:max_size 50M
        pm2 set pm2-logrotate:retain 14
        pm2 set pm2-logrotate:compress true
        pm2 save

        # Check PM2 process and logs
        pm2 list
        pm2 logs rustdesk-hbbs --lines 100
        pm2 logs rustdesk-hbbr --lines 100
        ```

        If using NSSM instead of PM2:
        - Enable NSSM online rotation and set `AppRotateBytes` (for example `104857600` = 100MB).
        """
    ).strip()


def _windows_to_linux_migration(windows_dir: str, linux_data_dir: str) -> str:
    return dedent(
        f"""
        ## Migration: Windows Server -> Linux Server

        Goal:
        - Preserve server identity and clients by migrating key files first.

        Step 1: Stop old Windows services/processes
        ```powershell
        pm2 stop rustdesk-hbbs
        pm2 stop rustdesk-hbbr
        # Or stop NSSM services from services.msc / nssm stop <service>
        ```

        Step 2: Backup critical files from old Windows host
        ```powershell
        $DataDir = Join-Path "{windows_dir}" "data"
        $Backup = "C:\\rustdesk-migration-backup"
        New-Item -ItemType Directory -Force -Path $Backup | Out-Null

        Copy-Item (Join-Path $DataDir "id_ed25519") $Backup -Force -ErrorAction SilentlyContinue
        Copy-Item (Join-Path $DataDir "id_ed25519.pub") $Backup -Force -ErrorAction SilentlyContinue
        Copy-Item (Join-Path $DataDir "db_v2.sqlite3*") $Backup -Force -ErrorAction SilentlyContinue
        Copy-Item (Join-Path $DataDir "db.sqlite3*") $Backup -Force -ErrorAction SilentlyContinue
        ```

        Step 3: Copy backup package to Linux host (SCP/WinSCP)
        ```bash
        # run on Windows terminal if OpenSSH client is installed
        scp -r C:/rustdesk-migration-backup user@<LINUX_HOST_IP>:/tmp/rustdesk-migration-backup
        ```

        Step 4: Restore files on Linux
        ```bash
        set -euo pipefail
        sudo install -d -m 0755 {linux_data_dir}
        sudo cp -av /tmp/rustdesk-migration-backup/* {linux_data_dir}/

        sudo chown root:root {linux_data_dir}/id_ed25519 {linux_data_dir}/id_ed25519.pub || true
        sudo chmod 600 {linux_data_dir}/id_ed25519 || true
        sudo chmod 644 {linux_data_dir}/id_ed25519.pub || true

        # If WAL files exist, keep ownership consistent
        sudo chown root:root {linux_data_dir}/db*.sqlite3* || true
        ```

        Step 5: Start Linux services and verify key consistency
        ```bash
        sudo systemctl restart rustdesk-hbbs rustdesk-hbbr
        sudo systemctl status rustdesk-hbbs --no-pager
        sudo systemctl status rustdesk-hbbr --no-pager

        # Verify public key used by new server
        cat {linux_data_dir}/id_ed25519.pub
        ```

        Migration checklist:
        - Must migrate: `id_ed25519`, `id_ed25519.pub`.
        - Usually migrate: `db_v2.sqlite3` (and `-wal`, `-shm` when present).
        - Do not migrate logs unless needed for audit.
        - Open the same ports on Linux before cutover.
        """
    ).strip()


def _source_notes() -> str:
    source_lines = "\n".join(f"- {name}: {url}" for name, url in SOURCES.items())
    return (
        "## Source Notes\n\n"
        "This guide aligns with RustDesk official docs and upstream repositories:\n"
        f"{source_lines}"
    )
