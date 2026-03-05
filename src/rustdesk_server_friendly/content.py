"""Guide content generator for RustDesk self-host operations."""

from __future__ import annotations

from textwrap import dedent

SUPPORTED_TARGETS = ("linux", "windows", "cross")
SUPPORTED_TOPICS = ("deploy", "logs", "service", "migrate", "all")
SUPPORTED_MIGRATION_OS = ("linux", "windows")

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
    migration_source_os: str = "windows",
    migration_target_os: str = "linux",
    migration_source_windows_dir: str = r"C:\RustDesk-Server",
    migration_target_windows_dir: str = r"C:\RustDesk-Server",
    migration_source_linux_data_dir: str = "/var/lib/rustdesk-server",
    migration_target_linux_data_dir: str = "/var/lib/rustdesk-server",
) -> str:
    """Render markdown guidance for a specific target/topic pair."""
    target = target.lower().strip()
    topic = topic.lower().strip()
    migration_source_os = migration_source_os.lower().strip()
    migration_target_os = migration_target_os.lower().strip()

    if target not in SUPPORTED_TARGETS:
        raise ValueError(f"Unsupported target: {target}")
    if topic not in SUPPORTED_TOPICS:
        raise ValueError(f"Unsupported topic: {topic}")
    if migration_source_os not in SUPPORTED_MIGRATION_OS:
        raise ValueError(f"Unsupported migration source OS: {migration_source_os}")
    if migration_target_os not in SUPPORTED_MIGRATION_OS:
        raise ValueError(f"Unsupported migration target OS: {migration_target_os}")

    host = host.strip() or "<PUBLIC_HOST_OR_IP>"

    sections: list[str] = []
    sections.append(
        _title(
            target=target,
            topic=topic,
            migration_source_os=migration_source_os,
            migration_target_os=migration_target_os,
        )
    )

    if topic == "migrate":
        sections.append(
            _migration_guide(
                source_os=migration_source_os,
                target_os=migration_target_os,
                source_windows_dir=migration_source_windows_dir,
                target_windows_dir=migration_target_windows_dir,
                source_linux_data_dir=migration_source_linux_data_dir,
                target_linux_data_dir=migration_target_linux_data_dir,
            )
        )
        sections.append(_source_notes())
        return "\n\n".join(sections).strip() + "\n"

    if target == "linux":
        if topic in ("deploy", "all"):
            sections.append(_linux_deploy(host, linux_install_dir, linux_data_dir, linux_log_dir))
        if topic in ("service", "all"):
            sections.append(_linux_service(host, linux_install_dir, linux_data_dir, linux_log_dir))
        if topic in ("logs", "all"):
            sections.append(_linux_log_limits(linux_log_dir))

    elif target == "windows":
        if topic in ("deploy", "all"):
            sections.append(_windows_deploy(host, windows_dir))
        if topic in ("service", "all"):
            sections.append(_windows_service(host, windows_dir))
        if topic in ("logs", "all"):
            sections.append(_windows_log_limits())

    else:
        sections.append(
            "`cross` target is for migration mode. Use `--topic migrate` with migration source/target OS options."
        )

    sections.append(_source_notes())
    return "\n\n".join(sections).strip() + "\n"


def _title(target: str, topic: str, migration_source_os: str, migration_target_os: str) -> str:
    lines = [
        "# RustDesk Server Friendly Guide",
        "",
        f"- Target: `{target}`",
        f"- Topic: `{topic}`",
    ]
    if topic == "migrate":
        lines.append(f"- Migration Pair: `{migration_source_os} -> {migration_target_os}`")
    return "\n".join(lines)


def _linux_deploy(host: str, install_dir: str, data_dir: str, log_dir: str) -> str:
    return dedent(
        f"""
        ## Linux CLI Deploy (Binary, Idempotent)

        Pull methods (choose one if you only want download):
        - `wget`: `wget -O rustdesk-server.zip "https://github.com/rustdesk/rustdesk-server/releases/download/<TAG>/rustdesk-server-linux-amd64.zip"`
        - `curl`: `curl -fL -o rustdesk-server.zip "https://github.com/rustdesk/rustdesk-server/releases/download/<TAG>/rustdesk-server-linux-amd64.zip"`
        - Full script below supports `DOWNLOAD_TOOL=auto|wget|curl` and auto-fallback.

        ```bash
        set -euo pipefail

        INSTALL_DIR="{install_dir}"
        DATA_DIR="{data_dir}"
        LOG_DIR="{log_dir}"
        FORCE_REINSTALL="${{FORCE_REINSTALL:-0}}"
        DOWNLOAD_TOOL="${{DOWNLOAD_TOOL:-auto}}"  # auto|curl|wget
        RUSTDESK_RELEASE_TAG="${{RUSTDESK_RELEASE_TAG:-}}"   # optional pin, e.g. 1.1.15
        RUSTDESK_ZIP_SHA256="${{RUSTDESK_ZIP_SHA256:-}}"     # optional integrity check

        choose_downloader() {{
          case "$DOWNLOAD_TOOL" in
            curl|wget) echo "$DOWNLOAD_TOOL" ;;
            auto)
              if command -v curl >/dev/null 2>&1; then echo curl; return; fi
              if command -v wget >/dev/null 2>&1; then echo wget; return; fi
              echo ""
              ;;
            *) echo "" ;;
          esac
        }}

        download_to() {{
          url="$1"
          out="$2"
          tool="$(choose_downloader)"
          [ -n "$tool" ] || {{ echo "[STOP] No downloader available"; return 1; }}
          if [ "$tool" = "curl" ]; then
            curl -fL "$url" -o "$out"
          else
            wget -O "$out" "$url"
          fi
        }}

        fetch_text() {{
          url="$1"
          tool="$(choose_downloader)"
          [ -n "$tool" ] || {{ echo "[STOP] No downloader available"; return 1; }}
          if [ "$tool" = "curl" ]; then
            curl -fsSL "$url"
          else
            wget -qO- "$url"
          fi
        }}

        verify_sha256() {{
          file="$1"
          expected="$2"
          if [ -z "$expected" ]; then
            return 0
          fi
          if command -v sha256sum >/dev/null 2>&1; then
            echo "$expected  $file" | sha256sum -c -
          elif command -v shasum >/dev/null 2>&1; then
            got="$(shasum -a 256 "$file" | awk '{{print $1}}')"
            [ "$got" = "$expected" ]
          else
            echo "[STOP] No sha256 tool found (need sha256sum or shasum) for integrity check."
            return 1
          fi
        }}

        if [ -x "$INSTALL_DIR/bin/hbbs" ] && [ -x "$INSTALL_DIR/bin/hbbr" ]; then
          echo "[SKIP] RustDesk binaries already exist at $INSTALL_DIR/bin"
        else
          if [ "$FORCE_REINSTALL" != "1" ] && {{ [ -e "$INSTALL_DIR/bin/hbbs" ] || [ -e "$INSTALL_DIR/bin/hbbr" ]; }}; then
            echo "[STOP] Partial install detected. Set FORCE_REINSTALL=1 to overwrite existing binaries."
            exit 1
          fi

          if command -v apt-get >/dev/null 2>&1; then
            sudo apt-get update
            sudo apt-get install -y curl wget unzip
          elif command -v dnf >/dev/null 2>&1; then
            sudo dnf install -y curl wget unzip
          elif command -v yum >/dev/null 2>&1; then
            sudo yum install -y curl wget unzip
          else
            echo "[STOP] Supported package manager (apt/dnf/yum) not found."
            exit 1
          fi

          if [ -z "$(choose_downloader)" ]; then
            echo "[STOP] Neither curl nor wget available."
            exit 1
          fi

          sudo install -d -m 0755 "$INSTALL_DIR/bin" "$DATA_DIR" "$LOG_DIR"

          RELEASE_JSON="$(fetch_text "https://api.github.com/repos/rustdesk/rustdesk-server/releases/latest")"
          LATEST_TAG="$(printf '%s' "$RELEASE_JSON" | awk -F '"' '/"tag_name":/{{print $4; exit}}')"
          TAG="${{RUSTDESK_RELEASE_TAG:-$LATEST_TAG}}"

          ARCH=$(uname -m)
          case "$ARCH" in
            x86_64) PKG_ARCH="amd64" ;;
            aarch64|arm64) PKG_ARCH="arm64v8" ;;
            armv7l) PKG_ARCH="armv7" ;;
            i386|i686) PKG_ARCH="i386" ;;
            *) echo "[STOP] Unsupported arch: $ARCH"; exit 1 ;;
          esac

          ASSET_NAME="rustdesk-server-linux-${{PKG_ARCH}}.zip"
          ASSET_URL="https://github.com/rustdesk/rustdesk-server/releases/download/${{TAG}}/${{ASSET_NAME}}"

          TMPDIR=$(mktemp -d)
          trap 'rm -rf "$TMPDIR"' EXIT
          ZIP_PATH="$TMPDIR/rustdesk-server.zip"

          download_to "$ASSET_URL" "$ZIP_PATH"
          verify_sha256 "$ZIP_PATH" "$RUSTDESK_ZIP_SHA256"
          unzip -q "$ZIP_PATH" -d "$TMPDIR"

          HBBS=$(find "$TMPDIR" -type f -name hbbs | head -n1)
          HBBR=$(find "$TMPDIR" -type f -name hbbr | head -n1)
          [ -n "$HBBS" ] && [ -n "$HBBR" ]

          sudo install -m 0755 "$HBBS" "$INSTALL_DIR/bin/hbbs"
          sudo install -m 0755 "$HBBR" "$INSTALL_DIR/bin/hbbr"
          echo "[OK] Binaries installed into $INSTALL_DIR/bin"
          "$INSTALL_DIR/bin/hbbs" -h >/dev/null 2>&1 || true
          "$INSTALL_DIR/bin/hbbr" -h >/dev/null 2>&1 || true
        fi

        # Optional firewall setup (idempotent)
        if command -v ufw >/dev/null 2>&1; then
          sudo ufw allow 21115:21118/tcp || true
          sudo ufw allow 21116/udp || true
        fi

        echo "Next: apply service setup section."
        ```
        """
    ).strip()


def _linux_service(host: str, install_dir: str, data_dir: str, log_dir: str) -> str:
    return dedent(
        f"""
        ## Linux Service Install (systemd, Idempotent)

        ```bash
        set -euo pipefail

        HBBS_UNIT="/etc/systemd/system/rustdesk-hbbs.service"
        HBBR_UNIT="/etc/systemd/system/rustdesk-hbbr.service"

        if [ ! -f "$HBBS_UNIT" ]; then
          sudo tee "$HBBS_UNIT" >/dev/null <<'UNIT'
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
          echo "[OK] Created $HBBS_UNIT"
        else
          echo "[SKIP] $HBBS_UNIT already exists"
        fi

        if [ ! -f "$HBBR_UNIT" ]; then
          sudo tee "$HBBR_UNIT" >/dev/null <<'UNIT'
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
          echo "[OK] Created $HBBR_UNIT"
        else
          echo "[SKIP] $HBBR_UNIT already exists"
        fi

        sudo systemctl daemon-reload

        if ! sudo systemctl is-enabled --quiet rustdesk-hbbs; then
          sudo systemctl enable rustdesk-hbbs
        fi
        if ! sudo systemctl is-enabled --quiet rustdesk-hbbr; then
          sudo systemctl enable rustdesk-hbbr
        fi

        if sudo systemctl is-active --quiet rustdesk-hbbs; then
          sudo systemctl restart rustdesk-hbbs
        else
          sudo systemctl start rustdesk-hbbs
        fi

        if sudo systemctl is-active --quiet rustdesk-hbbr; then
          sudo systemctl restart rustdesk-hbbr
        else
          sudo systemctl start rustdesk-hbbr
        fi

        sudo systemctl status rustdesk-hbbs --no-pager
        sudo systemctl status rustdesk-hbbr --no-pager
        sudo ss -lntup | grep -E '21115|21116|21117|21118' || true
        ```
        """
    ).strip()


def _linux_log_limits(log_dir: str) -> str:
    return dedent(
        f"""
        ## Linux Log Limits (Idempotent)

        ```bash
        set -euo pipefail

        LOGROTATE_FILE="/etc/logrotate.d/rustdesk-server"
        JOURNALD_FILE="/etc/systemd/journald.conf.d/rustdesk.conf"

        if [ ! -f "$LOGROTATE_FILE" ]; then
          sudo tee "$LOGROTATE_FILE" >/dev/null <<'CONF'
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
          echo "[OK] Created $LOGROTATE_FILE"
        else
          echo "[SKIP] $LOGROTATE_FILE already exists"
        fi

        sudo install -d /etc/systemd/journald.conf.d
        if [ ! -f "$JOURNALD_FILE" ]; then
          sudo tee "$JOURNALD_FILE" >/dev/null <<'CONF'
        [Journal]
        SystemMaxUse=500M
        RuntimeMaxUse=200M
        MaxRetentionSec=14day
        CONF
          echo "[OK] Created $JOURNALD_FILE"
        else
          echo "[SKIP] $JOURNALD_FILE already exists"
        fi

        sudo systemctl restart systemd-journald
        sudo logrotate -f "$LOGROTATE_FILE" || true
        ```
        """
    ).strip()


def _windows_deploy(host: str, windows_dir: str) -> str:
    _ = host
    return dedent(
        f"""
        ## Windows CLI Deploy (PowerShell, Idempotent)

        Pull methods:
        - `Invoke-WebRequest` (default in script)
        - `curl.exe -L` fallback
        - `gh release download` (install `GitHub.cli` via `winget`)

        ```powershell
        $ErrorActionPreference = "Stop"

        $Root = "{windows_dir}"
        $Bin = Join-Path $Root "bin"
        $Data = Join-Path $Root "data"
        $Logs = Join-Path $Root "logs"
        $ForceReinstall = ($env:FORCE_REINSTALL -eq "1")
        $DownloadMethod = ($env:DOWNLOAD_METHOD | ForEach-Object {{ $_.ToLower() }})  # auto|invokewebrequest|curl|gh
        if ([string]::IsNullOrWhiteSpace($DownloadMethod)) {{ $DownloadMethod = "auto" }}
        $PinnedTag = $env:RUSTDESK_RELEASE_TAG
        $ExpectedSha256 = $env:RUSTDESK_ZIP_SHA256

        New-Item -ItemType Directory -Force -Path $Bin, $Data, $Logs | Out-Null

        $HbbsExe = Join-Path $Bin "hbbs.exe"
        $HbbrExe = Join-Path $Bin "hbbr.exe"

        if ((Test-Path $HbbsExe) -and (Test-Path $HbbrExe)) {{
            Write-Host "[SKIP] RustDesk binaries already exist at $Bin"
            return
        }}

        if (-not $ForceReinstall -and ((Test-Path $HbbsExe) -or (Test-Path $HbbrExe))) {{
            throw "[STOP] Partial install detected. Set FORCE_REINSTALL=1 before rerunning."
        }}

        function Test-Command([string]$Name) {{
            return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
        }}

        $Release = Invoke-RestMethod "https://api.github.com/repos/rustdesk/rustdesk-server/releases/latest"
        $Tag = if ($PinnedTag) {{ $PinnedTag }} else {{ $Release.tag_name }}
        $AssetName = "rustdesk-server-windows-x86_64-unsigned.zip"
        $Asset = $Release.assets | Where-Object name -eq $AssetName | Select-Object -First 1
        $AssetUrl = if ($Asset) {{ $Asset.browser_download_url }} else {{ "https://github.com/rustdesk/rustdesk-server/releases/download/$Tag/$AssetName" }}

        $ZipPath = Join-Path $env:TEMP "rustdesk-server.zip"
        if (Test-Path $ZipPath) {{ Remove-Item $ZipPath -Force }}

        function Download-RustDeskZip([string]$Method, [string]$Url, [string]$TagName, [string]$OutFile) {{
            switch ($Method) {{
                "invokewebrequest" {{
                    Invoke-WebRequest -Uri $Url -OutFile $OutFile
                    return
                }}
                "curl" {{
                    if (-not (Test-Command "curl.exe")) {{ throw "curl.exe not found" }}
                    & curl.exe -fL $Url -o $OutFile
                    return
                }}
                "gh" {{
                    if (-not (Test-Command "gh")) {{
                        if (Test-Command "winget") {{
                            winget install GitHub.cli --accept-source-agreements --accept-package-agreements
                        }} else {{
                            throw "gh not found and winget unavailable"
                        }}
                    }}
                    gh release download $TagName --repo rustdesk/rustdesk-server --pattern $AssetName --output $OutFile --clobber
                    return
                }}
                "auto" {{
                    try {{
                        Invoke-WebRequest -Uri $Url -OutFile $OutFile
                        return
                    }} catch {{
                        if (Test-Command "curl.exe") {{
                            & curl.exe -fL $Url -o $OutFile
                            return
                        }}
                        if (Test-Command "gh") {{
                            gh release download $TagName --repo rustdesk/rustdesk-server --pattern $AssetName --output $OutFile --clobber
                            return
                        }}
                        throw "No download method succeeded (Invoke-WebRequest/curl.exe/gh)"
                    }}
                }}
                default {{
                    throw "Unsupported DOWNLOAD_METHOD: $Method"
                }}
            }}
        }}

        Download-RustDeskZip -Method $DownloadMethod -Url $AssetUrl -TagName $Tag -OutFile $ZipPath

        if ($ExpectedSha256) {{
            $Actual = (Get-FileHash -Path $ZipPath -Algorithm SHA256).Hash.ToLower()
            if ($Actual -ne $ExpectedSha256.ToLower()) {{
                throw "[STOP] SHA256 mismatch for downloaded zip"
            }}
        }}

        Expand-Archive -Path $ZipPath -DestinationPath $Bin -Force

        $hbbs = Get-ChildItem -Path $Bin -Filter hbbs.exe -Recurse | Select-Object -First 1
        $hbbr = Get-ChildItem -Path $Bin -Filter hbbr.exe -Recurse | Select-Object -First 1

        if (-not $hbbs -or -not $hbbr) {{
            throw "[STOP] hbbs.exe or hbbr.exe not found after extraction."
        }}

        Copy-Item $hbbs.FullName $HbbsExe -Force
        Copy-Item $hbbr.FullName $HbbrExe -Force
        Write-Host "[OK] Binaries installed into $Bin"
        & $HbbsExe --help *> $null
        & $HbbrExe --help *> $null
        ```
        """
    ).strip()


def _windows_service(host: str, windows_dir: str) -> str:
    return dedent(
        f"""
        ## Windows Service Install (PM2, Idempotent)

        ```powershell
        $ErrorActionPreference = "Stop"

        function Test-Command([string]$Name) {{
            return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
        }}

        if (-not (Test-Command "node")) {{
            if (-not (Test-Command "winget")) {{
                throw "[STOP] Node.js is missing and winget is not available. Install Node.js LTS manually first."
            }}
            winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements
        }}

        if (-not (Test-Command "pm2")) {{
            npm install -g pm2 pm2-windows-startup pm2-logrotate
        }}

        if (Test-Command "pm2-startup") {{
            pm2-startup install
        }}

        function Test-Pm2Process([string]$Name) {{
            try {{
                $items = pm2 jlist | ConvertFrom-Json
                foreach ($item in $items) {{
                    if ($item.name -eq $Name) {{ return $true }}
                }}
                return $false
            }} catch {{
                return $false
            }}
        }}

        $Bin = Join-Path "{windows_dir}" "bin"
        Set-Location $Bin

        if (Test-Pm2Process "rustdesk-hbbs") {{
            Write-Host "[SKIP] PM2 process rustdesk-hbbs already exists"
        }} else {{
            pm2 start .\\hbbs.exe --name rustdesk-hbbs -- -r {host}:21117
        }}

        if (Test-Pm2Process "rustdesk-hbbr") {{
            Write-Host "[SKIP] PM2 process rustdesk-hbbr already exists"
        }} else {{
            pm2 start .\\hbbr.exe --name rustdesk-hbbr
        }}

        pm2 save
        pm2 list
        Test-NetConnection -ComputerName 127.0.0.1 -Port 21116
        Test-NetConnection -ComputerName 127.0.0.1 -Port 21117
        ```

        Alternative GUI-style service install (NSSM):
        - Run `nssm install` to open GUI and register `hbbs.exe` / `hbbr.exe`.
        - In NSSM I/O tab, point stdout/stderr to `{windows_dir}\\logs\\*.log`.
        """
    ).strip()


def _windows_log_limits() -> str:
    return dedent(
        """
        ## Windows Log Limits (Idempotent)

        ```powershell
        $ErrorActionPreference = "Stop"

        if (-not (Get-Command pm2 -ErrorAction SilentlyContinue)) {
            throw "[STOP] pm2 is not installed. Apply service setup first."
        }

        $hasLogRotate = $false
        try {
            $null = pm2 conf pm2-logrotate:max_size 2>$null
            if ($LASTEXITCODE -eq 0) { $hasLogRotate = $true }
        } catch {
            $hasLogRotate = $false
        }

        if (-not $hasLogRotate) {
            pm2 install pm2-logrotate
        } else {
            Write-Host "[SKIP] pm2-logrotate already installed"
        }

        pm2 set pm2-logrotate:max_size 50M
        pm2 set pm2-logrotate:retain 14
        pm2 set pm2-logrotate:compress true
        pm2 save

        pm2 list
        pm2 logs rustdesk-hbbs --lines 50
        pm2 logs rustdesk-hbbr --lines 50
        ```

        If using NSSM instead of PM2:
        - Enable NSSM online rotation and set `AppRotateBytes` (for example `104857600` = 100MB).
        """
    ).strip()


def _migration_guide(
    source_os: str,
    target_os: str,
    source_windows_dir: str,
    target_windows_dir: str,
    source_linux_data_dir: str,
    target_linux_data_dir: str,
) -> str:
    pair = (source_os, target_os)
    if pair == ("windows", "linux"):
        return _migration_windows_to_linux(
            source_windows_dir=source_windows_dir,
            target_linux_data_dir=target_linux_data_dir,
        )
    if pair == ("linux", "windows"):
        return _migration_linux_to_windows(
            source_linux_data_dir=source_linux_data_dir,
            target_windows_dir=target_windows_dir,
        )
    if pair == ("linux", "linux"):
        return _migration_linux_to_linux(
            source_linux_data_dir=source_linux_data_dir,
            target_linux_data_dir=target_linux_data_dir,
        )
    if pair == ("windows", "windows"):
        return _migration_windows_to_windows(
            source_windows_dir=source_windows_dir,
            target_windows_dir=target_windows_dir,
        )
    raise ValueError(f"Unsupported migration pair: {source_os} -> {target_os}")


def _migration_windows_to_linux(source_windows_dir: str, target_linux_data_dir: str) -> str:
    body = dedent(
        f"""
        ## Migration: Windows -> Linux (Guided + Idempotent)

        Step 1: Stop source services on Windows
        ```powershell
        pm2 stop rustdesk-hbbs 2>$null
        pm2 stop rustdesk-hbbr 2>$null
        # Or stop NSSM services if PM2 is not used.
        ```

        Step 2: Build source backup package on Windows (auto-detect source data dir)
        ```powershell
        $DefaultSourceData = Join-Path "{source_windows_dir}" "data"
        $BackupRoot = "C:\\rustdesk-migration-backup"
        $Bundle = Join-Path $BackupRoot "bundle"
        $Zip = Join-Path $BackupRoot "rustdesk-migration-backup.zip"

        function Resolve-RustDeskDataDir {{
            param([string]$Fallback)

            if ($env:RUSTDESK_SOURCE_DATA_DIR -and (Test-Path $env:RUSTDESK_SOURCE_DATA_DIR)) {{
                return $env:RUSTDESK_SOURCE_DATA_DIR
            }}

            try {{
                $items = pm2 jlist | ConvertFrom-Json
                foreach ($item in $items) {{
                    if ($item.name -in @("rustdesk-hbbs", "hbbs")) {{
                        $cwd = $item.pm2_env.pm_cwd
                        if ($cwd -and (Test-Path $cwd)) {{
                            if ((Test-Path (Join-Path $cwd "id_ed25519")) -or (Test-Path (Join-Path $cwd "db_v2.sqlite3"))) {{
                                return $cwd
                            }}
                            $fromParent = Join-Path (Split-Path $cwd -Parent) "data"
                            if (Test-Path $fromParent) {{ return $fromParent }}
                        }}
                    }}
                }}
            }} catch {{}}

            foreach ($svc in @("rustdesk-hbbs", "hbbs", "rustdesksignal")) {{
                try {{
                    $svcParams = Get-ItemProperty -Path "HKLM:\\SYSTEM\\CurrentControlSet\\Services\\$svc\\Parameters" -ErrorAction Stop
                    if ($svcParams.AppDirectory) {{
                        if (Test-Path (Join-Path $svcParams.AppDirectory "id_ed25519")) {{ return $svcParams.AppDirectory }}
                        $fromSvc = Join-Path $svcParams.AppDirectory "data"
                        if (Test-Path $fromSvc) {{ return $fromSvc }}
                    }}
                }} catch {{}}
            }}

            foreach ($candidate in @($Fallback, "C:\\RustDesk-Server\\data", "C:\\rustdesk-server\\data", "C:\\Program Files\\RustDesk Server\\data")) {{
                if ($candidate -and (Test-Path $candidate)) {{ return $candidate }}
            }}
            return $Fallback
        }}

        $SourceData = Resolve-RustDeskDataDir -Fallback $DefaultSourceData
        Write-Host "[INFO] Source data dir: $SourceData"

        New-Item -ItemType Directory -Force -Path $Bundle | Out-Null
        if (Test-Path $Zip) {{ Remove-Item $Zip -Force }}

        $Patterns = @("id_ed25519", "id_ed25519.pub", "db_v2.sqlite3*", "db.sqlite3*")
        foreach ($Pattern in $Patterns) {{
            Get-ChildItem -Path (Join-Path $SourceData $Pattern) -ErrorAction SilentlyContinue |
                Copy-Item -Destination $Bundle -Force
        }}

        if (-not (Get-ChildItem -Path $Bundle -File -ErrorAction SilentlyContinue)) {{
            throw "[STOP] No migration files found under $SourceData"
        }}

        Compress-Archive -Path (Join-Path $Bundle "*") -DestinationPath $Zip -Force
        Write-Host "[OK] Backup package created: $Zip"
        ```

        Step 3: Transfer package to Linux target
        ```bash
        scp C:/rustdesk-migration-backup/rustdesk-migration-backup.zip user@<TARGET_LINUX_HOST>:/tmp/
        ```

        Step 4: Restore on Linux target (auto-detect target data dir, overwrite-protected)
        ```bash
        set -euo pipefail

        DEFAULT_TARGET_DATA_DIR="{target_linux_data_dir}"
        ARCHIVE="/tmp/rustdesk-migration-backup.zip"
        ALLOW_OVERWRITE="${{ALLOW_OVERWRITE:-0}}"

        resolve_rustdesk_data_dir() {{
          fallback="$1"
          if [ -n "${{RUSTDESK_TARGET_DATA_DIR:-}}" ] && [ -d "${{RUSTDESK_TARGET_DATA_DIR}}" ]; then
            echo "${{RUSTDESK_TARGET_DATA_DIR}}"
            return
          fi

          for unit in rustdesk-hbbs rustdesk-hbbr hbbs rustdesksignal; do
            wd=$(systemctl show -p WorkingDirectory --value "$unit" 2>/dev/null || true)
            if [ -n "$wd" ] && [ "$wd" != "-" ] && [ -d "$wd" ]; then
              echo "$wd"
              return
            fi
          done

          for proc in hbbs rustdesksignal; do
            pid=$(pgrep -xo "$proc" 2>/dev/null || true)
            if [ -n "$pid" ] && [ -d "/proc/$pid/cwd" ]; then
              readlink -f "/proc/$pid/cwd"
              return
            fi
          done

          for d in "$fallback" /var/lib/rustdesk-server /opt/rustdesk-server /opt/rustdesk; do
            if [ -d "$d" ]; then
              echo "$d"
              return
            fi
          done
          echo "$fallback"
        }}

        TARGET_DATA_DIR="${{RUSTDESK_TARGET_DATA_DIR:-$(resolve_rustdesk_data_dir "$DEFAULT_TARGET_DATA_DIR")}}"
        echo "[INFO] Target data dir: $TARGET_DATA_DIR"

        [ -f "$ARCHIVE" ] || {{ echo "[STOP] $ARCHIVE not found"; exit 1; }}

        if [ "$ALLOW_OVERWRITE" != "1" ] && {{ [ -f "$TARGET_DATA_DIR/id_ed25519" ] || [ -f "$TARGET_DATA_DIR/id_ed25519.pub" ]; }}; then
          echo "[STOP] Destination keys exist. Set ALLOW_OVERWRITE=1 to replace."
          exit 1
        fi

        sudo install -d -m 0755 "$TARGET_DATA_DIR"
        TMPDIR=$(mktemp -d)
        trap 'rm -rf "$TMPDIR"' EXIT
        unzip -o "$ARCHIVE" -d "$TMPDIR"
        sudo cp -av "$TMPDIR"/* "$TARGET_DATA_DIR"/

        sudo chown root:root "$TARGET_DATA_DIR"/id_ed25519 "$TARGET_DATA_DIR"/id_ed25519.pub 2>/dev/null || true
        sudo chmod 600 "$TARGET_DATA_DIR"/id_ed25519 2>/dev/null || true
        sudo chmod 644 "$TARGET_DATA_DIR"/id_ed25519.pub 2>/dev/null || true
        sudo chown root:root "$TARGET_DATA_DIR"/db*.sqlite3* 2>/dev/null || true

        echo "[OK] Restore completed"
        ```

        Step 5: Start Linux services
        ```bash
        sudo systemctl restart rustdesk-hbbs rustdesk-hbbr
        sudo systemctl status rustdesk-hbbs --no-pager
        sudo systemctl status rustdesk-hbbr --no-pager
        ```
        """
    ).strip()
    return f"{body}\n\n{_migration_checklist()}"


def _migration_linux_to_windows(source_linux_data_dir: str, target_windows_dir: str) -> str:
    body = dedent(
        f"""
        ## Migration: Linux -> Windows (Guided + Idempotent)

        Step 1: Stop source services on Linux
        ```bash
        sudo systemctl stop rustdesk-hbbs rustdesk-hbbr
        ```

        Step 2: Build source backup package on Linux (auto-detect source data dir)
        ```bash
        set -euo pipefail

        DEFAULT_SOURCE_DATA_DIR="{source_linux_data_dir}"
        BACKUP_DIR="/tmp/rustdesk-migration-backup"
        ARCHIVE="/tmp/rustdesk-migration-backup.tgz"

        resolve_rustdesk_data_dir() {{
          fallback="$1"
          if [ -n "${{RUSTDESK_SOURCE_DATA_DIR:-}}" ] && [ -d "${{RUSTDESK_SOURCE_DATA_DIR}}" ]; then
            echo "${{RUSTDESK_SOURCE_DATA_DIR}}"
            return
          fi

          for unit in rustdesk-hbbs rustdesk-hbbr hbbs rustdesksignal; do
            wd=$(systemctl show -p WorkingDirectory --value "$unit" 2>/dev/null || true)
            if [ -n "$wd" ] && [ "$wd" != "-" ] && [ -d "$wd" ]; then
              echo "$wd"
              return
            fi
          done

          for proc in hbbs rustdesksignal; do
            pid=$(pgrep -xo "$proc" 2>/dev/null || true)
            if [ -n "$pid" ] && [ -d "/proc/$pid/cwd" ]; then
              readlink -f "/proc/$pid/cwd"
              return
            fi
          done

          for d in "$fallback" /var/lib/rustdesk-server /opt/rustdesk-server /opt/rustdesk; do
            if [ -d "$d" ]; then
              echo "$d"
              return
            fi
          done
          echo "$fallback"
        }}

        SOURCE_DATA_DIR="${{RUSTDESK_SOURCE_DATA_DIR:-$(resolve_rustdesk_data_dir "$DEFAULT_SOURCE_DATA_DIR")}}"
        echo "[INFO] Source data dir: $SOURCE_DATA_DIR"

        rm -rf "$BACKUP_DIR"
        mkdir -p "$BACKUP_DIR"

        for f in id_ed25519 id_ed25519.pub; do
          [ -f "$SOURCE_DATA_DIR/$f" ] && cp -a "$SOURCE_DATA_DIR/$f" "$BACKUP_DIR/"
        done

        shopt -s nullglob
        for f in "$SOURCE_DATA_DIR"/db_v2.sqlite3* "$SOURCE_DATA_DIR"/db.sqlite3*; do
          cp -a "$f" "$BACKUP_DIR/"
        done
        shopt -u nullglob

        if ! ls -A "$BACKUP_DIR" >/dev/null 2>&1; then
          echo "[STOP] No migration files found under $SOURCE_DATA_DIR"
          exit 1
        fi

        tar -C "$BACKUP_DIR" -czf "$ARCHIVE" .
        echo "[OK] Backup package created: $ARCHIVE"
        ```

        Step 3: Transfer package to Windows target
        ```bash
        scp /tmp/rustdesk-migration-backup.tgz Administrator@<TARGET_WINDOWS_HOST>:C:/rustdesk-migration-backup/
        ```

        Step 4: Restore on Windows target (auto-detect target data dir, overwrite-protected)
        ```powershell
        $ErrorActionPreference = "Stop"

        $DefaultTargetData = Join-Path "{target_windows_dir}" "data"
        $Archive = "C:\\rustdesk-migration-backup\\rustdesk-migration-backup.tgz"
        $AllowOverwrite = ($env:ALLOW_OVERWRITE -eq "1")

        function Resolve-RustDeskDataDir {{
            param([string]$Fallback)

            if ($env:RUSTDESK_TARGET_DATA_DIR -and (Test-Path $env:RUSTDESK_TARGET_DATA_DIR)) {{
                return $env:RUSTDESK_TARGET_DATA_DIR
            }}

            try {{
                $items = pm2 jlist | ConvertFrom-Json
                foreach ($item in $items) {{
                    if ($item.name -in @("rustdesk-hbbs", "hbbs")) {{
                        $cwd = $item.pm2_env.pm_cwd
                        if ($cwd -and (Test-Path $cwd)) {{
                            if ((Test-Path (Join-Path $cwd "id_ed25519")) -or (Test-Path (Join-Path $cwd "db_v2.sqlite3"))) {{
                                return $cwd
                            }}
                            $fromParent = Join-Path (Split-Path $cwd -Parent) "data"
                            if (Test-Path $fromParent) {{ return $fromParent }}
                        }}
                    }}
                }}
            }} catch {{}}

            foreach ($svc in @("rustdesk-hbbs", "hbbs", "rustdesksignal")) {{
                try {{
                    $svcParams = Get-ItemProperty -Path "HKLM:\\SYSTEM\\CurrentControlSet\\Services\\$svc\\Parameters" -ErrorAction Stop
                    if ($svcParams.AppDirectory) {{
                        if (Test-Path (Join-Path $svcParams.AppDirectory "id_ed25519")) {{ return $svcParams.AppDirectory }}
                        $fromSvc = Join-Path $svcParams.AppDirectory "data"
                        if (Test-Path $fromSvc) {{ return $fromSvc }}
                    }}
                }} catch {{}}
            }}

            foreach ($candidate in @($Fallback, "C:\\RustDesk-Server\\data", "C:\\rustdesk-server\\data", "C:\\Program Files\\RustDesk Server\\data")) {{
                if ($candidate -and (Test-Path $candidate)) {{ return $candidate }}
            }}
            return $Fallback
        }}

        $TargetData = Resolve-RustDeskDataDir -Fallback $DefaultTargetData
        Write-Host "[INFO] Target data dir: $TargetData"

        if (-not (Test-Path $Archive)) {{
            throw "[STOP] Archive not found: $Archive"
        }}

        New-Item -ItemType Directory -Force -Path $TargetData | Out-Null

        if (-not $AllowOverwrite -and ((Test-Path (Join-Path $TargetData "id_ed25519")) -or (Test-Path (Join-Path $TargetData "id_ed25519.pub")))) {{
            throw "[STOP] Destination keys already exist. Set ALLOW_OVERWRITE=1 to replace."
        }}

        tar -xzf $Archive -C $TargetData
        Write-Host "[OK] Restore completed"
        ```

        Step 5: Start Windows services
        ```powershell
        pm2 start rustdesk-hbbs 2>$null
        pm2 start rustdesk-hbbr 2>$null
        # Or start NSSM services if PM2 is not used.
        ```
        """
    ).strip()
    return f"{body}\n\n{_migration_checklist()}"


def _migration_linux_to_linux(source_linux_data_dir: str, target_linux_data_dir: str) -> str:
    body = dedent(
        f"""
        ## Migration: Linux -> Linux (Guided + Idempotent)

        Step 1: Stop source services
        ```bash
        sudo systemctl stop rustdesk-hbbs rustdesk-hbbr
        ```

        Step 2: Build source backup package (auto-detect source data dir)
        ```bash
        set -euo pipefail

        DEFAULT_SOURCE_DATA_DIR="{source_linux_data_dir}"
        BACKUP_DIR="/tmp/rustdesk-migration-backup"
        ARCHIVE="/tmp/rustdesk-migration-backup.tgz"

        resolve_rustdesk_data_dir() {{
          fallback="$1"
          if [ -n "${{RUSTDESK_SOURCE_DATA_DIR:-}}" ] && [ -d "${{RUSTDESK_SOURCE_DATA_DIR}}" ]; then
            echo "${{RUSTDESK_SOURCE_DATA_DIR}}"
            return
          fi

          for unit in rustdesk-hbbs rustdesk-hbbr hbbs rustdesksignal; do
            wd=$(systemctl show -p WorkingDirectory --value "$unit" 2>/dev/null || true)
            if [ -n "$wd" ] && [ "$wd" != "-" ] && [ -d "$wd" ]; then
              echo "$wd"
              return
            fi
          done

          for proc in hbbs rustdesksignal; do
            pid=$(pgrep -xo "$proc" 2>/dev/null || true)
            if [ -n "$pid" ] && [ -d "/proc/$pid/cwd" ]; then
              readlink -f "/proc/$pid/cwd"
              return
            fi
          done

          for d in "$fallback" /var/lib/rustdesk-server /opt/rustdesk-server /opt/rustdesk; do
            if [ -d "$d" ]; then
              echo "$d"
              return
            fi
          done
          echo "$fallback"
        }}

        SOURCE_DATA_DIR="${{RUSTDESK_SOURCE_DATA_DIR:-$(resolve_rustdesk_data_dir "$DEFAULT_SOURCE_DATA_DIR")}}"
        echo "[INFO] Source data dir: $SOURCE_DATA_DIR"

        rm -rf "$BACKUP_DIR"
        mkdir -p "$BACKUP_DIR"

        for f in id_ed25519 id_ed25519.pub; do
          [ -f "$SOURCE_DATA_DIR/$f" ] && cp -a "$SOURCE_DATA_DIR/$f" "$BACKUP_DIR/"
        done

        shopt -s nullglob
        for f in "$SOURCE_DATA_DIR"/db_v2.sqlite3* "$SOURCE_DATA_DIR"/db.sqlite3*; do
          cp -a "$f" "$BACKUP_DIR/"
        done
        shopt -u nullglob

        if ! ls -A "$BACKUP_DIR" >/dev/null 2>&1; then
          echo "[STOP] No migration files found under $SOURCE_DATA_DIR"
          exit 1
        fi

        tar -C "$BACKUP_DIR" -czf "$ARCHIVE" .
        echo "[OK] Backup package created: $ARCHIVE"
        ```

        Step 3: Transfer package to target Linux
        ```bash
        scp /tmp/rustdesk-migration-backup.tgz user@<TARGET_LINUX_HOST>:/tmp/
        ```

        Step 4: Restore on target Linux (auto-detect target data dir, overwrite-protected)
        ```bash
        set -euo pipefail

        DEFAULT_TARGET_DATA_DIR="{target_linux_data_dir}"
        ARCHIVE="/tmp/rustdesk-migration-backup.tgz"
        ALLOW_OVERWRITE="${{ALLOW_OVERWRITE:-0}}"

        resolve_target_data_dir() {{
          fallback="$1"
          if [ -n "${{RUSTDESK_TARGET_DATA_DIR:-}}" ] && [ -d "${{RUSTDESK_TARGET_DATA_DIR}}" ]; then
            echo "${{RUSTDESK_TARGET_DATA_DIR}}"
            return
          fi

          for unit in rustdesk-hbbs rustdesk-hbbr hbbs rustdesksignal; do
            wd=$(systemctl show -p WorkingDirectory --value "$unit" 2>/dev/null || true)
            if [ -n "$wd" ] && [ "$wd" != "-" ] && [ -d "$wd" ]; then
              echo "$wd"
              return
            fi
          done

          for proc in hbbs rustdesksignal; do
            pid=$(pgrep -xo "$proc" 2>/dev/null || true)
            if [ -n "$pid" ] && [ -d "/proc/$pid/cwd" ]; then
              readlink -f "/proc/$pid/cwd"
              return
            fi
          done

          for d in "$fallback" /var/lib/rustdesk-server /opt/rustdesk-server /opt/rustdesk; do
            if [ -d "$d" ]; then
              echo "$d"
              return
            fi
          done
          echo "$fallback"
        }}

        TARGET_DATA_DIR="${{RUSTDESK_TARGET_DATA_DIR:-$(resolve_target_data_dir "$DEFAULT_TARGET_DATA_DIR")}}"
        echo "[INFO] Target data dir: $TARGET_DATA_DIR"

        [ -f "$ARCHIVE" ] || {{ echo "[STOP] $ARCHIVE not found"; exit 1; }}

        if [ "$ALLOW_OVERWRITE" != "1" ] && {{ [ -f "$TARGET_DATA_DIR/id_ed25519" ] || [ -f "$TARGET_DATA_DIR/id_ed25519.pub" ]; }}; then
          echo "[STOP] Destination keys exist. Set ALLOW_OVERWRITE=1 to replace."
          exit 1
        fi

        sudo install -d -m 0755 "$TARGET_DATA_DIR"
        sudo tar -C "$TARGET_DATA_DIR" -xzf "$ARCHIVE"

        sudo chown root:root "$TARGET_DATA_DIR"/id_ed25519 "$TARGET_DATA_DIR"/id_ed25519.pub 2>/dev/null || true
        sudo chmod 600 "$TARGET_DATA_DIR"/id_ed25519 2>/dev/null || true
        sudo chmod 644 "$TARGET_DATA_DIR"/id_ed25519.pub 2>/dev/null || true
        sudo chown root:root "$TARGET_DATA_DIR"/db*.sqlite3* 2>/dev/null || true

        echo "[OK] Restore completed"
        ```

        Step 5: Start target services
        ```bash
        sudo systemctl restart rustdesk-hbbs rustdesk-hbbr
        sudo systemctl status rustdesk-hbbs --no-pager
        sudo systemctl status rustdesk-hbbr --no-pager
        ```
        """
    ).strip()
    return f"{body}\n\n{_migration_checklist()}"


def _migration_windows_to_windows(source_windows_dir: str, target_windows_dir: str) -> str:
    body = dedent(
        f"""
        ## Migration: Windows -> Windows (Guided + Idempotent)

        Step 1: Stop source services
        ```powershell
        pm2 stop rustdesk-hbbs 2>$null
        pm2 stop rustdesk-hbbr 2>$null
        # Or stop NSSM services if PM2 is not used.
        ```

        Step 2: Build source backup package (auto-detect source data dir)
        ```powershell
        $DefaultSourceData = Join-Path "{source_windows_dir}" "data"
        $BackupRoot = "C:\\rustdesk-migration-backup"
        $Bundle = Join-Path $BackupRoot "bundle"
        $Zip = Join-Path $BackupRoot "rustdesk-migration-backup.zip"

        function Resolve-RustDeskDataDir {{
            param(
                [string]$Fallback,
                [string]$OverrideEnv
            )

            $overrideValue = $null
            if ($OverrideEnv) {{
                $entry = Get-Item -Path "Env:$OverrideEnv" -ErrorAction SilentlyContinue
                if ($entry) {{ $overrideValue = $entry.Value }}
            }}
            if ($overrideValue -and (Test-Path $overrideValue)) {{
                return $overrideValue
            }}

            try {{
                $items = pm2 jlist | ConvertFrom-Json
                foreach ($item in $items) {{
                    if ($item.name -in @("rustdesk-hbbs", "hbbs")) {{
                        $cwd = $item.pm2_env.pm_cwd
                        if ($cwd -and (Test-Path $cwd)) {{
                            if ((Test-Path (Join-Path $cwd "id_ed25519")) -or (Test-Path (Join-Path $cwd "db_v2.sqlite3"))) {{
                                return $cwd
                            }}
                            $fromParent = Join-Path (Split-Path $cwd -Parent) "data"
                            if (Test-Path $fromParent) {{ return $fromParent }}
                        }}
                    }}
                }}
            }} catch {{}}

            foreach ($svc in @("rustdesk-hbbs", "hbbs", "rustdesksignal")) {{
                try {{
                    $svcParams = Get-ItemProperty -Path "HKLM:\\SYSTEM\\CurrentControlSet\\Services\\$svc\\Parameters" -ErrorAction Stop
                    if ($svcParams.AppDirectory) {{
                        if (Test-Path (Join-Path $svcParams.AppDirectory "id_ed25519")) {{ return $svcParams.AppDirectory }}
                        $fromSvc = Join-Path $svcParams.AppDirectory "data"
                        if (Test-Path $fromSvc) {{ return $fromSvc }}
                    }}
                }} catch {{}}
            }}

            foreach ($candidate in @($Fallback, "C:\\RustDesk-Server\\data", "C:\\rustdesk-server\\data", "C:\\Program Files\\RustDesk Server\\data")) {{
                if ($candidate -and (Test-Path $candidate)) {{ return $candidate }}
            }}
            return $Fallback
        }}

        $SourceData = Resolve-RustDeskDataDir -Fallback $DefaultSourceData -OverrideEnv "RUSTDESK_SOURCE_DATA_DIR"
        Write-Host "[INFO] Source data dir: $SourceData"

        New-Item -ItemType Directory -Force -Path $Bundle | Out-Null
        if (Test-Path $Zip) {{ Remove-Item $Zip -Force }}

        $Patterns = @("id_ed25519", "id_ed25519.pub", "db_v2.sqlite3*", "db.sqlite3*")
        foreach ($Pattern in $Patterns) {{
            Get-ChildItem -Path (Join-Path $SourceData $Pattern) -ErrorAction SilentlyContinue |
                Copy-Item -Destination $Bundle -Force
        }}

        if (-not (Get-ChildItem -Path $Bundle -File -ErrorAction SilentlyContinue)) {{
            throw "[STOP] No migration files found under $SourceData"
        }}

        Compress-Archive -Path (Join-Path $Bundle "*") -DestinationPath $Zip -Force
        Write-Host "[OK] Backup package created: $Zip"
        ```

        Step 3: Transfer package to target Windows
        ```powershell
        # Option A: SMB copy
        Copy-Item C:/rustdesk-migration-backup/rustdesk-migration-backup.zip \\\\<TARGET_WINDOWS_HOST>\\c$\\rustdesk-migration-backup\\ -Force

        # Option B: SCP
        # scp C:/rustdesk-migration-backup/rustdesk-migration-backup.zip Administrator@<TARGET_WINDOWS_HOST>:C:/rustdesk-migration-backup/
        ```

        Step 4: Restore on target Windows (auto-detect target data dir, overwrite-protected)
        ```powershell
        $ErrorActionPreference = "Stop"

        $DefaultTargetData = Join-Path "{target_windows_dir}" "data"
        $Zip = "C:\\rustdesk-migration-backup\\rustdesk-migration-backup.zip"
        $AllowOverwrite = ($env:ALLOW_OVERWRITE -eq "1")

        $TargetData = Resolve-RustDeskDataDir -Fallback $DefaultTargetData -OverrideEnv "RUSTDESK_TARGET_DATA_DIR"
        Write-Host "[INFO] Target data dir: $TargetData"

        if (-not (Test-Path $Zip)) {{
            throw "[STOP] Package not found: $Zip"
        }}

        New-Item -ItemType Directory -Force -Path $TargetData | Out-Null

        if (-not $AllowOverwrite -and ((Test-Path (Join-Path $TargetData "id_ed25519")) -or (Test-Path (Join-Path $TargetData "id_ed25519.pub")))) {{
            throw "[STOP] Destination keys already exist. Set ALLOW_OVERWRITE=1 to replace."
        }}

        Expand-Archive -Path $Zip -DestinationPath $TargetData -Force
        Write-Host "[OK] Restore completed"
        ```

        Step 5: Start target services
        ```powershell
        pm2 start rustdesk-hbbs 2>$null
        pm2 start rustdesk-hbbr 2>$null
        # Or start NSSM services if PM2 is not used.
        ```
        """
    ).strip()
    return f"{body}\n\n{_migration_checklist()}"


def _migration_checklist() -> str:
    return dedent(
        """
        Migration checklist:
        - Must migrate: `id_ed25519`, `id_ed25519.pub`.
        - Usually migrate: `db_v2.sqlite3` (and `-wal`, `-shm` when present).
        - Do not migrate logs unless needed for audit.
        - Keep old server stopped during cutover to avoid data divergence.
        - For professional change control, record SHA256 of backup archive before and after transfer.
        - Paths are auto-detected from running service/PM2/NSSM data first; use `RUSTDESK_SOURCE_DATA_DIR` or `RUSTDESK_TARGET_DATA_DIR` to override.
        """
    ).strip()


def _source_notes() -> str:
    source_lines = "\n".join(f"- {name}: {url}" for name, url in SOURCES.items())
    return (
        "## Source Notes\n\n"
        "This guide aligns with RustDesk official docs and upstream repositories:\n"
        f"{source_lines}"
    )
