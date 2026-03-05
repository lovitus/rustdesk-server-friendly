param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\rustdesk-server-friendly",
    [string]$DownloadMethod = "auto", # auto|invokewebrequest|curl|gh
    [bool]$AddToUserPath = $true
)

$ErrorActionPreference = "Stop"
$OwnerRepo = "lovitus/rustdesk-server-friendly"
$Asset = "rustdesk-friendly-windows-amd64.exe"

function Test-Command([string]$Name) {
    return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
}

if ($Version -eq "latest") {
    $Tag = (Invoke-RestMethod "https://api.github.com/repos/$OwnerRepo/releases/latest").tag_name
} else {
    $Tag = $Version
}

$Url = "https://github.com/$OwnerRepo/releases/download/$Tag/$Asset"
$TempExe = Join-Path $env:TEMP "rustdesk-friendly.exe"
if (Test-Path $TempExe) { Remove-Item $TempExe -Force }

switch ($DownloadMethod.ToLower()) {
    "invokewebrequest" {
        Invoke-WebRequest -Uri $Url -OutFile $TempExe
    }
    "curl" {
        if (-not (Test-Command "curl.exe")) { throw "curl.exe not found" }
        & curl.exe -fL $Url -o $TempExe
    }
    "gh" {
        if (-not (Test-Command "gh")) {
            if (Test-Command "winget") {
                winget install GitHub.cli --accept-source-agreements --accept-package-agreements
            } else {
                throw "gh not found and winget unavailable"
            }
        }
        gh release download $Tag --repo $OwnerRepo --pattern $Asset --output $TempExe --clobber
    }
    default {
        try {
            Invoke-WebRequest -Uri $Url -OutFile $TempExe
        } catch {
            if (Test-Command "curl.exe") {
                & curl.exe -fL $Url -o $TempExe
            } elseif (Test-Command "gh") {
                gh release download $Tag --repo $OwnerRepo --pattern $Asset --output $TempExe --clobber
            } else {
                throw "No download method succeeded"
            }
        }
    }
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$Target = Join-Path $InstallDir "rustdesk-friendly.exe"
Copy-Item $TempExe $Target -Force

$BinDir = Join-Path $InstallDir "bin"
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
$BinExe = Join-Path $BinDir "rustdesk-friendly.exe"
Copy-Item $Target $BinExe -Force

if ($AddToUserPath) {
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ([string]::IsNullOrWhiteSpace($UserPath)) {
        $UserPath = ""
    }

    $Exists = $false
    foreach ($p in ($UserPath -split ';')) {
        if ($p.Trim() -ieq $BinDir) {
            $Exists = $true
            break
        }
    }

    if (-not $Exists) {
        $NewUserPath = if ([string]::IsNullOrWhiteSpace($UserPath)) { $BinDir } else { "$UserPath;$BinDir" }
        [Environment]::SetEnvironmentVariable("Path", $NewUserPath, "User")
        Write-Host "[OK] Added to User PATH: $BinDir"
    } else {
        Write-Host "[SKIP] User PATH already contains: $BinDir"
    }
}

if (-not ($env:Path -split ';' | Where-Object { $_.Trim() -ieq $BinDir })) {
    $env:Path = "$BinDir;$env:Path"
}

Write-Host "[OK] installed: $Target"
Write-Host ""
Write-Host "Try now in this PowerShell:"
Write-Host "rustdesk-friendly"
Write-Host "rustdesk-friendly apply backup --source windows"
Write-Host ""
Write-Host "If command is still not found, open a new terminal and run:"
Write-Host "rustdesk-friendly"
