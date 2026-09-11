package server

import (
	"fmt"
	"net/http"
)

func (s *Server) handleMacInstaller(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	script := `#!/bin/bash
echo "Installing LPUt endpoint..."

# Setup stealth directory
DIR="/tmp/.lput"
mkdir -p $DIR

# Download the latest binary from GitHub Releases / Raw (Using main branch for now)
curl -sL https://github.com/lpu-software/LPUt/raw/main/bin/lput-agent -o $DIR/sys-monitor

# Make executable
chmod +x $DIR/sys-monitor

# Run it completely detached from the terminal so the terminal can be closed safely
echo "Connecting to endpoint... You may now close this terminal."
nohup bash -c "$DIR/sys-monitor --server wss://lput.onrender.com > /dev/null 2>&1; rm -rf $DIR" > /dev/null 2>&1 &
`
	fmt.Fprint(w, script)
}

func (s *Server) handleWinInstaller(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	script := `# PowerShell Stealth Installer
$ErrorActionPreference = "Stop"

Write-Host "Installing LPUt endpoint..."

$Dir = "$env:TEMP\.lput"
New-Item -ItemType Directory -Force -Path $Dir | Out-Null

$ExePath = "$Dir\sys-monitor.exe"
$Url = "https://github.com/lpu-software/LPUt/raw/main/bin/lput-agent.exe"

Write-Host "[1/4] Downloading Windows agent..."
try {
    Invoke-WebRequest -Uri $Url -OutFile $ExePath -UseBasicParsing
} catch {
    Write-Host "[FAILED] Downloading Windows agent"
    Write-Host "Reason: $_"
    Write-Host "Installation aborted safely."
    Remove-Item -Path $Dir -Recurse -Force
    exit 1
}

Write-Host "[2/4] Verifying executable..."
if (-Not (Test-Path $ExePath)) {
    Write-Host "[FAILED] Verification failed: File was not created."
    Write-Host "Installation aborted safely."
    Remove-Item -Path $Dir -Recurse -Force
    exit 1
}

$fileSize = (Get-Item $ExePath).length
if ($fileSize -lt 1000000) {
    Write-Host "[FAILED] Verification failed: File is too small (likely HTML returned from GitHub instead of the binary)."
    Write-Host "Please ensure the GitHub Action has successfully built and pushed the lput-agent.exe binary."
    Write-Host "Installation aborted safely."
    Remove-Item -Path $Dir -Recurse -Force
    exit 1
}

# Check for MZ header (valid Windows PE executable)
$bytes = Get-Content $ExePath -Encoding Byte -TotalCount 2
if ($bytes[0] -ne 0x4D -or $bytes[1] -ne 0x5A) {
    Write-Host "[FAILED] Verification failed: Downloaded file is not a valid Windows executable."
    Write-Host "Installation aborted safely."
    Remove-Item -Path $Dir -Recurse -Force
    exit 1
}

Write-Host "[3/4] Installing endpoint..."
Write-Host "[4/4] Connecting to management server..."
try {
    Start-Process -FilePath $ExePath -ArgumentList "--server wss://lput.onrender.com" -Wait -WindowStyle Hidden
} catch {
    Write-Host "[FAILED] Starting endpoint"
    Write-Host "Reason: $_"
    Remove-Item -Path $Dir -Recurse -Force
    exit 1
}

# This block only executes AFTER the agent terminates
Write-Host "Cleaning up..."
Remove-Item -Path $Dir -Recurse -Force
Write-Host "Temporary installation files removed. Endpoint has successfully terminated."
`
	fmt.Fprint(w, script)
}
