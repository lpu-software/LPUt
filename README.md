# LPUt (Enterprise Remote Management)

LPUt is a lightweight, authorized remote-management and screen-sharing tool designed for enterprise IT support. It consists of a centralized server and cross-platform agents (macOS and Windows).

## Features
- **Cross-Platform:** Agents for macOS and Windows.
- **Screen Streaming:** Uses Apple `ScreenCaptureKit` on macOS and `BitBlt` on Windows to stream compressed JPEGs to the server.
- **Remote Control:** Full remote mouse and keyboard support.
- **File Transfer:** Drag-and-drop file upload to target devices.
- **SEB Compatibility Mode (macOS):** Intelligently detects if Safe Exam Browser is running and safely restricts screen capture to prevent `WindowServer` lockout errors.
- **Stealth Installer:** Background agent installer that survives terminal closures (via `nohup`).

## Architecture
1. **Server (`lput-server`)**: Written in Go. Hosts the web dashboard and coordinates WebSocket connections between the operator and the agents.
2. **Agent (`lput-agent`)**: Written in Go. Runs on the target computer in the background. Connects outbound to the server over WebSockets.
3. **Dashboard**: Vanilla JS and HTML served by the `lput-server`. 

---

## 1. How to Build

You will need [Go](https://go.dev/doc/install) installed.

### Build the Server
```bash
# Navigate to the project root
cd /path/to/LPUt

# Build the server binary
go build -o bin/lput-server ./cmd/lput-server
```

### Build the Agent (macOS or Windows)
```bash
# From the project root
go build -o bin/lput-agent ./cmd/lput-agent
```

*Note: For macOS, `CGO_ENABLED=1` is required to compile the Objective-C ScreenCaptureKit bindings.*

---

## 2. How to Run

### A. Run the Server (Dashboard)
The server must be running to receive agent connections and serve the dashboard.

```bash
# Run on port 8443
./bin/lput-server --port 8443
```

By default, you can now access the dashboard at:
`http://localhost:8443/dashboard`

*(In production, LPUt is typically deployed to a cloud provider like Render which maps this port to HTTPS).*

### B. Run the Agent (Target Computer)
To deploy the agent on a target machine, run the following command on the target computer. 
*Note: You must replace `link.com` with your actual server URL.*

**For macOS:**
```bash
curl -sL https://link.com/mac | bash
```

**For Windows (Run in PowerShell):**
```powershell
irm https://link.com/win | iex
```

Once installed, the agent will instantly connect to the server and appear on your Dashboard.

### C. Run the Agent Diagnostic Tool (macOS)
If you need to check the OS permissions, Code Signing, or Safe Exam Browser (SEB) detection status on a target Mac, you can run the diagnostic command:

```bash
./bin/lput-agent diagnose-macos
```

## Dashboard Operations
- **Connecting:** Click "Connect" on a device card to open the remote control interface.
- **Removing a Device:** Click the red trash can icon on a device card. This instantly commands the remote agent to self-destruct and deletes the device from the server's tracking memory.
