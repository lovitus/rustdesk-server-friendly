param(
    [switch]$ResetVenv
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $RepoRoot

Write-Host "[INFO] Repo root: $RepoRoot"

if (-not (Test-Path (Join-Path $RepoRoot "pyproject.toml"))) {
    throw "[STOP] pyproject.toml not found. Please run this script from rustdesk-server-friendly repo."
}

$VenvDir = Join-Path $RepoRoot ".venv"
if ($ResetVenv -and (Test-Path $VenvDir)) {
    Write-Host "[INFO] Removing existing .venv"
    Remove-Item -Recurse -Force $VenvDir
}

if (-not (Test-Path $VenvDir)) {
    if (Get-Command py -ErrorAction SilentlyContinue) {
        Write-Host "[INFO] Creating venv with: py -3 -m venv .venv"
        py -3 -m venv .venv
    } elseif (Get-Command python -ErrorAction SilentlyContinue) {
        Write-Host "[INFO] Creating venv with: python -m venv .venv"
        python -m venv .venv
    } else {
        throw "[STOP] Python launcher not found. Install Python 3.9+ first."
    }
} else {
    Write-Host "[SKIP] .venv already exists"
}

$VenvPython = Join-Path $VenvDir "Scripts\python.exe"
if (-not (Test-Path $VenvPython)) {
    throw "[STOP] venv python not found: $VenvPython"
}

Write-Host "[INFO] Installing/Updating packaging tools"
& $VenvPython -m pip install --upgrade pip setuptools wheel

Write-Host "[INFO] Installing rustdesk-server-friendly"
& $VenvPython -m pip install -e .

$Exe = Join-Path $VenvDir "Scripts\rustdesk-friendly.exe"
if (Test-Path $Exe) {
    Write-Host "[OK] Installed successfully"
    Write-Host "Run wizard with: $Exe"
} else {
    Write-Host "[WARN] Entry exe not found. Run via: $VenvPython -m rustdesk_server_friendly"
}
