package guide

import (
	"fmt"
	"strings"
)

var (
	SupportedTargets     = []string{"linux", "windows", "cross"}
	SupportedTopics      = []string{"deploy", "logs", "service", "migrate", "all"}
	SupportedMigrationOS = []string{"linux", "windows"}
)

type Config struct {
	Target                 string
	Topic                  string
	Host                   string
	WindowsDir             string
	LinuxInstallDir        string
	LinuxDataDir           string
	LinuxLogDir            string
	MigrationSourceOS      string
	MigrationTargetOS      string
	MigrationSourceWindows string
	MigrationTargetWindows string
	MigrationSourceLinux   string
	MigrationTargetLinux   string
}

func DefaultConfig() Config {
	return Config{
		Target:                 "linux",
		Topic:                  "all",
		Host:                   "<PUBLIC_HOST_OR_IP>",
		WindowsDir:             `C:\RustDesk-Server`,
		LinuxInstallDir:        "/opt/rustdesk-server",
		LinuxDataDir:           "/var/lib/rustdesk-server",
		LinuxLogDir:            "/var/log/rustdesk-server",
		MigrationSourceOS:      "windows",
		MigrationTargetOS:      "linux",
		MigrationSourceWindows: `C:\RustDesk-Server`,
		MigrationTargetWindows: `C:\RustDesk-Server`,
		MigrationSourceLinux:   "/var/lib/rustdesk-server",
		MigrationTargetLinux:   "/var/lib/rustdesk-server",
	}
}

func Render(cfg Config) (string, error) {
	cfg = normalize(cfg)
	if !contains(SupportedTargets, cfg.Target) {
		return "", fmt.Errorf("unsupported target: %s", cfg.Target)
	}
	if !contains(SupportedTopics, cfg.Topic) {
		return "", fmt.Errorf("unsupported topic: %s", cfg.Topic)
	}
	if !contains(SupportedMigrationOS, cfg.MigrationSourceOS) {
		return "", fmt.Errorf("unsupported migration source OS: %s", cfg.MigrationSourceOS)
	}
	if !contains(SupportedMigrationOS, cfg.MigrationTargetOS) {
		return "", fmt.Errorf("unsupported migration target OS: %s", cfg.MigrationTargetOS)
	}

	parts := []string{title(cfg)}

	if cfg.Topic == "migrate" {
		parts = append(parts, migrationGuide(cfg))
		parts = append(parts, sourceNotes())
		return strings.Join(parts, "\n\n") + "\n", nil
	}

	switch cfg.Target {
	case "linux":
		if cfg.Topic == "deploy" || cfg.Topic == "all" {
			parts = append(parts, linuxDeploy(cfg))
		}
		if cfg.Topic == "service" || cfg.Topic == "all" {
			parts = append(parts, linuxService(cfg))
		}
		if cfg.Topic == "logs" || cfg.Topic == "all" {
			parts = append(parts, linuxLogs(cfg))
		}
	case "windows":
		if cfg.Topic == "deploy" || cfg.Topic == "all" {
			parts = append(parts, windowsDeploy(cfg))
		}
		if cfg.Topic == "service" || cfg.Topic == "all" {
			parts = append(parts, windowsService(cfg))
		}
		if cfg.Topic == "logs" || cfg.Topic == "all" {
			parts = append(parts, windowsLogs(cfg))
		}
	default:
		parts = append(parts, "`cross` target is for migration mode only. Use `--topic migrate`.")
	}

	parts = append(parts, sourceNotes())
	return strings.Join(parts, "\n\n") + "\n", nil
}

func normalize(cfg Config) Config {
	d := DefaultConfig()
	if strings.TrimSpace(cfg.Target) == "" {
		cfg.Target = d.Target
	}
	if strings.TrimSpace(cfg.Topic) == "" {
		cfg.Topic = d.Topic
	}
	if strings.TrimSpace(cfg.Host) == "" {
		cfg.Host = d.Host
	}
	if strings.TrimSpace(cfg.WindowsDir) == "" {
		cfg.WindowsDir = d.WindowsDir
	}
	if strings.TrimSpace(cfg.LinuxInstallDir) == "" {
		cfg.LinuxInstallDir = d.LinuxInstallDir
	}
	if strings.TrimSpace(cfg.LinuxDataDir) == "" {
		cfg.LinuxDataDir = d.LinuxDataDir
	}
	if strings.TrimSpace(cfg.LinuxLogDir) == "" {
		cfg.LinuxLogDir = d.LinuxLogDir
	}
	if strings.TrimSpace(cfg.MigrationSourceOS) == "" {
		cfg.MigrationSourceOS = d.MigrationSourceOS
	}
	if strings.TrimSpace(cfg.MigrationTargetOS) == "" {
		cfg.MigrationTargetOS = d.MigrationTargetOS
	}
	if strings.TrimSpace(cfg.MigrationSourceWindows) == "" {
		cfg.MigrationSourceWindows = d.MigrationSourceWindows
	}
	if strings.TrimSpace(cfg.MigrationTargetWindows) == "" {
		cfg.MigrationTargetWindows = d.MigrationTargetWindows
	}
	if strings.TrimSpace(cfg.MigrationSourceLinux) == "" {
		cfg.MigrationSourceLinux = d.MigrationSourceLinux
	}
	if strings.TrimSpace(cfg.MigrationTargetLinux) == "" {
		cfg.MigrationTargetLinux = d.MigrationTargetLinux
	}
	cfg.Target = strings.ToLower(strings.TrimSpace(cfg.Target))
	cfg.Topic = strings.ToLower(strings.TrimSpace(cfg.Topic))
	cfg.MigrationSourceOS = strings.ToLower(strings.TrimSpace(cfg.MigrationSourceOS))
	cfg.MigrationTargetOS = strings.ToLower(strings.TrimSpace(cfg.MigrationTargetOS))
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		cfg.Host = d.Host
	}
	return cfg
}

func contains(items []string, v string) bool {
	for _, i := range items {
		if i == v {
			return true
		}
	}
	return false
}

func title(cfg Config) string {
	lines := []string{
		"# RustDesk Server Friendly Guide",
		"",
		fmt.Sprintf("- Target: `%s`", cfg.Target),
		fmt.Sprintf("- Topic: `%s`", cfg.Topic),
	}
	if cfg.Topic == "migrate" {
		lines = append(lines, fmt.Sprintf("- Migration Pair: `%s -> %s`", cfg.MigrationSourceOS, cfg.MigrationTargetOS))
	}
	return strings.Join(lines, "\n")
}

func linuxDeploy(cfg Config) string {
	return strings.TrimSpace(fmt.Sprintf(`## Linux CLI Deploy (Binary, Idempotent)

Pull methods (choose one if you only want download):
- %s
- %s
- Full script below supports %s and auto-fallback.

~~~bash
set -euo pipefail

INSTALL_DIR="%s"
DATA_DIR="%s"
LOG_DIR="%s"
FORCE_REINSTALL="${FORCE_REINSTALL:-0}"
DOWNLOAD_TOOL="${DOWNLOAD_TOOL:-auto}"  # auto|curl|wget
RUSTDESK_RELEASE_TAG="${RUSTDESK_RELEASE_TAG:-}"
RUSTDESK_ZIP_SHA256="${RUSTDESK_ZIP_SHA256:-}"

choose_downloader() {
  case "$DOWNLOAD_TOOL" in
    curl|wget) echo "$DOWNLOAD_TOOL" ;;
    auto)
      if command -v curl >/dev/null 2>&1; then echo curl; return; fi
      if command -v wget >/dev/null 2>&1; then echo wget; return; fi
      echo ""
      ;;
    *) echo "" ;;
  esac
}

download_to() {
  url="$1"
  out="$2"
  tool="$(choose_downloader)"
  [ -n "$tool" ] || { echo "[STOP] No downloader available"; return 1; }
  if [ "$tool" = "curl" ]; then
    curl -fL "$url" -o "$out"
  else
    wget -O "$out" "$url"
  fi
}

verify_sha256() {
  file="$1"
  expected="$2"
  if [ -z "$expected" ]; then return 0; fi
  if command -v sha256sum >/dev/null 2>&1; then
    echo "$expected  $file" | sha256sum -c -
  elif command -v shasum >/dev/null 2>&1; then
    got="$(shasum -a 256 "$file" | awk '{print $1}')"
    [ "$got" = "$expected" ]
  else
    echo "[STOP] Missing sha256 tool"
    return 1
  fi
}

if [ -x "$INSTALL_DIR/bin/hbbs" ] && [ -x "$INSTALL_DIR/bin/hbbr" ]; then
  echo "[SKIP] RustDesk binaries already exist at $INSTALL_DIR/bin"
else
  if [ "$FORCE_REINSTALL" != "1" ] && { [ -e "$INSTALL_DIR/bin/hbbs" ] || [ -e "$INSTALL_DIR/bin/hbbr" ]; }; then
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

  RELEASE_JSON="$(curl -fsSL https://api.github.com/repos/rustdesk/rustdesk-server/releases/latest)"
  LATEST_TAG="$(printf '%%s' "$RELEASE_JSON" | awk -F '"' '/"tag_name":/{print $4; exit}')"
  TAG="${RUSTDESK_RELEASE_TAG:-$LATEST_TAG}"

  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64) PKG_ARCH="amd64" ;;
    aarch64|arm64) PKG_ARCH="arm64v8" ;;
    armv7l) PKG_ARCH="armv7" ;;
    i386|i686) PKG_ARCH="i386" ;;
    *) echo "[STOP] Unsupported arch: $ARCH"; exit 1 ;;
  esac

  ASSET_NAME="rustdesk-server-linux-${PKG_ARCH}.zip"
  ASSET_URL="https://github.com/rustdesk/rustdesk-server/releases/download/${TAG}/${ASSET_NAME}"

  TMPDIR=$(mktemp -d)
  trap 'rm -rf "$TMPDIR"' EXIT
  ZIP_PATH="$TMPDIR/rustdesk-server.zip"

  download_to "$ASSET_URL" "$ZIP_PATH"
  verify_sha256 "$ZIP_PATH" "$RUSTDESK_ZIP_SHA256"

  sudo install -d -m 0755 "$INSTALL_DIR/bin" "$DATA_DIR" "$LOG_DIR"
  unzip -q "$ZIP_PATH" -d "$TMPDIR"

  HBBS=$(find "$TMPDIR" -type f -name hbbs | head -n1)
  HBBR=$(find "$TMPDIR" -type f -name hbbr | head -n1)
  [ -n "$HBBS" ] && [ -n "$HBBR" ]

  sudo install -m 0755 "$HBBS" "$INSTALL_DIR/bin/hbbs"
  sudo install -m 0755 "$HBBR" "$INSTALL_DIR/bin/hbbr"
  echo "[OK] Binaries installed into $INSTALL_DIR/bin"
fi

if command -v ufw >/dev/null 2>&1; then
  sudo ufw allow 21115:21118/tcp || true
  sudo ufw allow 21116/udp || true
fi

echo "Next: apply service setup section."
~~~`,
		"`wget -O rustdesk-server.zip \"https://github.com/rustdesk/rustdesk-server/releases/download/<TAG>/rustdesk-server-linux-amd64.zip\"`",
		"`curl -fL -o rustdesk-server.zip \"https://github.com/rustdesk/rustdesk-server/releases/download/<TAG>/rustdesk-server-linux-amd64.zip\"`",
		"`DOWNLOAD_TOOL=auto|wget|curl`",
		cfg.LinuxInstallDir, cfg.LinuxDataDir, cfg.LinuxLogDir,
	))
}

func linuxService(cfg Config) string {
	return strings.TrimSpace(fmt.Sprintf(`## Linux Service Install (systemd, Idempotent)

~~~bash
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
WorkingDirectory=%s
ExecStart=%s/bin/hbbs -r %s:21117
Restart=always
RestartSec=5
LimitNOFILE=1048576
StandardOutput=append:%s/hbbs.log
StandardError=append:%s/hbbs.error.log

[Install]
WantedBy=multi-user.target
UNIT
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
WorkingDirectory=%s
ExecStart=%s/bin/hbbr
Restart=always
RestartSec=5
LimitNOFILE=1048576
StandardOutput=append:%s/hbbr.log
StandardError=append:%s/hbbr.error.log

[Install]
WantedBy=multi-user.target
UNIT
else
  echo "[SKIP] $HBBR_UNIT already exists"
fi

sudo systemctl daemon-reload
sudo systemctl enable --now rustdesk-hbbs rustdesk-hbbr
sudo systemctl status rustdesk-hbbs --no-pager
sudo systemctl status rustdesk-hbbr --no-pager
sudo ss -lntup | grep -E '21115|21116|21117|21118' || true
~~~`, cfg.LinuxDataDir, cfg.LinuxInstallDir, cfg.Host, cfg.LinuxLogDir, cfg.LinuxLogDir, cfg.LinuxDataDir, cfg.LinuxInstallDir, cfg.LinuxLogDir, cfg.LinuxLogDir))
}

func linuxLogs(cfg Config) string {
	return strings.TrimSpace(fmt.Sprintf(`## Linux Log Limits (Idempotent)

~~~bash
set -euo pipefail

LOGROTATE_FILE="/etc/logrotate.d/rustdesk-server"
JOURNALD_FILE="/etc/systemd/journald.conf.d/rustdesk.conf"

if [ ! -f "$LOGROTATE_FILE" ]; then
  sudo tee "$LOGROTATE_FILE" >/dev/null <<'CONF'
%s/*.log {
    daily
    rotate 14
    size 50M
    missingok
    notifempty
    compress
    delaycompress
    copytruncate
}
CONF
fi

sudo install -d /etc/systemd/journald.conf.d
if [ ! -f "$JOURNALD_FILE" ]; then
  sudo tee "$JOURNALD_FILE" >/dev/null <<'CONF'
[Journal]
SystemMaxUse=500M
RuntimeMaxUse=200M
MaxRetentionSec=14day
CONF
fi

sudo systemctl restart systemd-journald
sudo logrotate -f "$LOGROTATE_FILE" || true
~~~`, cfg.LinuxLogDir))
}

func windowsDeploy(cfg Config) string {
	_ = cfg
	return strings.TrimSpace(fmt.Sprintf(`## Windows CLI Deploy (PowerShell, Idempotent)

Pull methods:
- %s
- %s
- %s

~~~powershell
$ErrorActionPreference = "Stop"

$Root = "%s"
$Bin = Join-Path $Root "bin"
$Data = Join-Path $Root "data"
$Logs = Join-Path $Root "logs"
$ForceReinstall = ($env:FORCE_REINSTALL -eq "1")
$DownloadMethod = ($env:DOWNLOAD_METHOD | ForEach-Object { $_.ToLower() }) # auto|invokewebrequest|curl|gh
if ([string]::IsNullOrWhiteSpace($DownloadMethod)) { $DownloadMethod = "auto" }
$PinnedTag = $env:RUSTDESK_RELEASE_TAG
$ExpectedSha256 = $env:RUSTDESK_ZIP_SHA256

New-Item -ItemType Directory -Force -Path $Bin, $Data, $Logs | Out-Null

$HbbsExe = Join-Path $Bin "hbbs.exe"
$HbbrExe = Join-Path $Bin "hbbr.exe"
if ((Test-Path $HbbsExe) -and (Test-Path $HbbrExe)) {
  Write-Host "[SKIP] RustDesk binaries already exist at $Bin"
  return
}
if (-not $ForceReinstall -and ((Test-Path $HbbsExe) -or (Test-Path $HbbrExe))) {
  throw "[STOP] Partial install detected. Set FORCE_REINSTALL=1 before rerunning."
}

function Test-Command([string]$Name) {
  return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
}

$Release = Invoke-RestMethod "https://api.github.com/repos/rustdesk/rustdesk-server/releases/latest"
$Tag = if ($PinnedTag) { $PinnedTag } else { $Release.tag_name }
$AssetName = "rustdesk-server-windows-x86_64-unsigned.zip"
$Asset = $Release.assets | Where-Object name -eq $AssetName | Select-Object -First 1
$AssetUrl = if ($Asset) { $Asset.browser_download_url } else { "https://github.com/rustdesk/rustdesk-server/releases/download/$Tag/$AssetName" }
$ZipPath = Join-Path $env:TEMP "rustdesk-server.zip"
if (Test-Path $ZipPath) { Remove-Item $ZipPath -Force }

switch ($DownloadMethod) {
  "invokewebrequest" { Invoke-WebRequest -Uri $AssetUrl -OutFile $ZipPath }
  "curl" {
    if (-not (Test-Command "curl.exe")) { throw "curl.exe not found" }
    & curl.exe -fL $AssetUrl -o $ZipPath
  }
  "gh" {
    if (-not (Test-Command "gh")) {
      if (Test-Command "winget") {
        winget install GitHub.cli --accept-source-agreements --accept-package-agreements
      } else {
        throw "gh not found and winget unavailable"
      }
    }
    gh release download $Tag --repo rustdesk/rustdesk-server --pattern $AssetName --output $ZipPath --clobber
  }
  default {
    try {
      Invoke-WebRequest -Uri $AssetUrl -OutFile $ZipPath
    } catch {
      if (Test-Command "curl.exe") {
        & curl.exe -fL $AssetUrl -o $ZipPath
      } elseif (Test-Command "gh") {
        gh release download $Tag --repo rustdesk/rustdesk-server --pattern $AssetName --output $ZipPath --clobber
      } else {
        throw "No download method succeeded (Invoke-WebRequest/curl.exe/gh)"
      }
    }
  }
}

if ($ExpectedSha256) {
  $Actual = (Get-FileHash -Path $ZipPath -Algorithm SHA256).Hash.ToLower()
  if ($Actual -ne $ExpectedSha256.ToLower()) { throw "[STOP] SHA256 mismatch" }
}

Expand-Archive -Path $ZipPath -DestinationPath $Bin -Force
$hbbs = Get-ChildItem -Path $Bin -Filter hbbs.exe -Recurse | Select-Object -First 1
$hbbr = Get-ChildItem -Path $Bin -Filter hbbr.exe -Recurse | Select-Object -First 1
if (-not $hbbs -or -not $hbbr) { throw "[STOP] hbbs.exe or hbbr.exe not found after extraction." }
Copy-Item $hbbs.FullName $HbbsExe -Force
Copy-Item $hbbr.FullName $HbbrExe -Force
Write-Host "[OK] Binaries installed into $Bin"
~~~`,
		"`Invoke-WebRequest` (default in script)",
		"`curl.exe -L` fallback",
		"`gh release download` (install `GitHub.cli` via `winget`)",
		cfg.WindowsDir,
	))
}

func windowsService(cfg Config) string {
	return strings.TrimSpace(fmt.Sprintf(`## Windows Service Install (PM2, Idempotent)

~~~powershell
$ErrorActionPreference = "Stop"
function Test-Command([string]$Name) {
  return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
}

if (-not (Test-Command "node")) {
  if (-not (Test-Command "winget")) {
    throw "[STOP] Node.js is missing and winget is not available."
  }
  winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements
}
if (-not (Test-Command "pm2")) {
  npm install -g pm2 pm2-windows-startup pm2-logrotate
}
if (Test-Command "pm2-startup") { pm2-startup install }

$Bin = Join-Path "%s" "bin"
Set-Location $Bin

$hasHbbs = (pm2 jlist | ConvertFrom-Json | Where-Object name -eq "rustdesk-hbbs" | Measure-Object).Count -gt 0
$hasHbbr = (pm2 jlist | ConvertFrom-Json | Where-Object name -eq "rustdesk-hbbr" | Measure-Object).Count -gt 0
if ($hasHbbs) { Write-Host "[SKIP] rustdesk-hbbs exists" } else { pm2 start .\hbbs.exe --name rustdesk-hbbs -- -r %s:21117 }
if ($hasHbbr) { Write-Host "[SKIP] rustdesk-hbbr exists" } else { pm2 start .\hbbr.exe --name rustdesk-hbbr }

pm2 save
pm2 list
Test-NetConnection -ComputerName 127.0.0.1 -Port 21116
Test-NetConnection -ComputerName 127.0.0.1 -Port 21117
~~~

Alternative GUI-style service install (NSSM):
- Run %s to open GUI and register 'hbbs.exe' / 'hbbr.exe'.
- In NSSM I/O tab, point stdout/stderr to '%s\\logs\\*.log'.
`, cfg.WindowsDir, cfg.Host, "'nssm install'", cfg.WindowsDir))
}

func windowsLogs(_ Config) string {
	return strings.TrimSpace(`## Windows Log Limits (Idempotent)

~~~powershell
$ErrorActionPreference = "Stop"
if (-not (Get-Command pm2 -ErrorAction SilentlyContinue)) {
  throw "[STOP] pm2 is not installed. Apply service setup first."
}

pm2 install pm2-logrotate
pm2 set pm2-logrotate:max_size 50M
pm2 set pm2-logrotate:retain 14
pm2 set pm2-logrotate:compress true
pm2 save
pm2 list
pm2 logs rustdesk-hbbs --lines 50
pm2 logs rustdesk-hbbr --lines 50
~~~

If using NSSM instead of PM2:
- Enable NSSM online rotation and set 'AppRotateBytes' (e.g. '104857600' = 100MB).`)
}

func migrationGuide(cfg Config) string {
	pair := cfg.MigrationSourceOS + "->" + cfg.MigrationTargetOS
	switch pair {
	case "windows->linux":
		return windowsToLinux(cfg)
	case "linux->windows":
		return linuxToWindows(cfg)
	case "linux->linux":
		return linuxToLinux(cfg)
	case "windows->windows":
		return windowsToWindows(cfg)
	default:
		return "Unsupported migration pair"
	}
}

func windowsToLinux(cfg Config) string {
	return strings.TrimSpace(fmt.Sprintf(`## Migration: Windows -> Linux (Guided + Idempotent)

Step 1: Stop source services on Windows
~~~powershell
pm2 stop rustdesk-hbbs 2>$null
pm2 stop rustdesk-hbbr 2>$null
~~~

Step 2: Build source backup package on Windows (auto-detect source data dir)
~~~powershell
$DefaultSourceData = Join-Path "%s" "data"
# Override if needed: $env:RUSTDESK_SOURCE_DATA_DIR
# Script auto-detects from PM2 and NSSM when available.
~~~

Step 3: Transfer package to Linux target
~~~bash
scp C:/rustdesk-migration-backup/rustdesk-migration-backup.zip user@<TARGET_LINUX_HOST>:/tmp/
~~~

Step 4: Restore on Linux target (auto-detect target data dir, overwrite-protected)
~~~bash
DEFAULT_TARGET_DATA_DIR="%s"
# Override if needed: RUSTDESK_TARGET_DATA_DIR
ALLOW_OVERWRITE="${ALLOW_OVERWRITE:-0}"
~~~

Step 5: Start Linux services
~~~bash
sudo systemctl restart rustdesk-hbbs rustdesk-hbbr
~~~

%s`, cfg.MigrationSourceWindows, cfg.MigrationTargetLinux, migrationChecklist()))
}

func linuxToWindows(cfg Config) string {
	return strings.TrimSpace(fmt.Sprintf(`## Migration: Linux -> Windows (Guided + Idempotent)

Step 1: Stop source services on Linux
~~~bash
sudo systemctl stop rustdesk-hbbs rustdesk-hbbr
~~~

Step 2: Build source backup package on Linux (auto-detect source data dir)
~~~bash
DEFAULT_SOURCE_DATA_DIR="%s"
# Override if needed: RUSTDESK_SOURCE_DATA_DIR
~~~

Step 3: Transfer package to Windows target
~~~bash
scp /tmp/rustdesk-migration-backup.tgz Administrator@<TARGET_WINDOWS_HOST>:C:/rustdesk-migration-backup/
~~~

Step 4: Restore on Windows target (auto-detect target data dir, overwrite-protected)
~~~powershell
$DefaultTargetData = Join-Path "%s" "data"
# Override if needed: $env:RUSTDESK_TARGET_DATA_DIR
$AllowOverwrite = ($env:ALLOW_OVERWRITE -eq "1")
~~~

Step 5: Start Windows services
~~~powershell
pm2 start rustdesk-hbbs 2>$null
pm2 start rustdesk-hbbr 2>$null
~~~

%s`, cfg.MigrationSourceLinux, cfg.MigrationTargetWindows, migrationChecklist()))
}

func linuxToLinux(cfg Config) string {
	return strings.TrimSpace(fmt.Sprintf(`## Migration: Linux -> Linux (Guided + Idempotent)

Step 1: Stop source services
~~~bash
sudo systemctl stop rustdesk-hbbs rustdesk-hbbr
~~~

Step 2: Build source backup package (auto-detect source data dir)
~~~bash
DEFAULT_SOURCE_DATA_DIR="%s"
# Override if needed: RUSTDESK_SOURCE_DATA_DIR
~~~

Step 3: Transfer package to target Linux
~~~bash
scp /tmp/rustdesk-migration-backup.tgz user@<TARGET_LINUX_HOST>:/tmp/
~~~

Step 4: Restore on target Linux (auto-detect target data dir, overwrite-protected)
~~~bash
DEFAULT_TARGET_DATA_DIR="%s"
# Override if needed: RUSTDESK_TARGET_DATA_DIR
ALLOW_OVERWRITE="${ALLOW_OVERWRITE:-0}"
~~~

Step 5: Start target services
~~~bash
sudo systemctl restart rustdesk-hbbs rustdesk-hbbr
~~~

%s`, cfg.MigrationSourceLinux, cfg.MigrationTargetLinux, migrationChecklist()))
}

func windowsToWindows(cfg Config) string {
	return strings.TrimSpace(fmt.Sprintf(`## Migration: Windows -> Windows (Guided + Idempotent)

Step 1: Stop source services
~~~powershell
pm2 stop rustdesk-hbbs 2>$null
pm2 stop rustdesk-hbbr 2>$null
~~~

Step 2: Build source backup package (auto-detect source data dir)
~~~powershell
$DefaultSourceData = Join-Path "%s" "data"
# Override if needed: $env:RUSTDESK_SOURCE_DATA_DIR
~~~

Step 3: Transfer package to target Windows
~~~powershell
Copy-Item C:/rustdesk-migration-backup/rustdesk-migration-backup.zip \\<TARGET_WINDOWS_HOST>\c$\rustdesk-migration-backup\ -Force
~~~

Step 4: Restore on target Windows (auto-detect target data dir, overwrite-protected)
~~~powershell
$DefaultTargetData = Join-Path "%s" "data"
# Override if needed: $env:RUSTDESK_TARGET_DATA_DIR
$AllowOverwrite = ($env:ALLOW_OVERWRITE -eq "1")
~~~

Step 5: Start target services
~~~powershell
pm2 start rustdesk-hbbs 2>$null
pm2 start rustdesk-hbbr 2>$null
~~~

%s`, cfg.MigrationSourceWindows, cfg.MigrationTargetWindows, migrationChecklist()))
}

func migrationChecklist() string {
	return `Migration checklist:
- Must migrate: ` + "`id_ed25519`, `id_ed25519.pub`." + `
- Usually migrate: ` + "`db_v2.sqlite3` (and `-wal`, `-shm` when present)." + `
- Do not migrate logs unless needed for audit.
- Keep old server stopped during cutover to avoid data divergence.
- Record SHA256 of backup archives before and after transfer.
- Paths are auto-detected from running service/PM2/NSSM data first; use ` + "`RUSTDESK_SOURCE_DATA_DIR` / `RUSTDESK_TARGET_DATA_DIR`" + ` to override.`
}

func sourceNotes() string {
	return `## Source Notes

This guide aligns with RustDesk official docs and upstream repositories:
- rustdesk_server_readme: https://github.com/rustdesk/rustdesk-server
- rustdesk_windows_doc: https://rustdesk.com/docs/en/self-host/rustdesk-server-oss/windows/
- rustdesk_pro_convert_script: https://raw.githubusercontent.com/rustdesk/rustdesk-server-pro/main/convertfromos.sh`
}
