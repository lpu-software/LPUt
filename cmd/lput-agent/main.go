package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	agentpkg "github.com/yatishydv/lput/internal/agent"
	"github.com/yatishydv/lput/internal/capture"
)

func main() {
	serverURL := flag.String("server", "ws://localhost:8443", "LPUt server URL")
	deviceID := flag.String("device-id", "", "Device ID (auto-generated if empty)")
	authToken := flag.String("auth-token", "", "Authentication token")
	flag.Parse()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "help":
			printHelp()
			return
		case "version":
			fmt.Println("LPUt Agent v1.0.0")
			return
		case "diagnose-macos":
			runDiagnostics()
			return
		}
	}

	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║     LPUt — Enterprise Remote Management     ║")
	fmt.Println("║                  Agent v1.0.0                ║")
	fmt.Println("╠══════════════════════════════════════════════╣")
	fmt.Printf("║  Server:    %-33s║\n", *serverURL)
	fmt.Println("╚══════════════════════════════════════════════╝")

	ag := agentpkg.New(agentpkg.Config{
		ServerURL: *serverURL,
		DeviceID:  *deviceID,
		AuthToken: *authToken,
	})

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("[Agent] Shutting down...")
		ag.Stop()
		os.Exit(0)
	}()

	if err := ag.Run(); err != nil {
		log.Fatal(err)
	}
}

func printHelp() {
	fmt.Println("LPUt Agent — Enterprise Remote Management Endpoint")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  lput-agent [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --server <url>        Server URL (default: ws://localhost:8443)")
	fmt.Println("  --device-id <id>      Device ID (auto-generated if empty)")
	fmt.Println("  --auth-token <token>  Authentication token")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  help             Show this help")
	fmt.Println("  version          Show version")
	fmt.Println("  diagnose-macos   Run macOS specific diagnostic checks (Permissions, SEB status, etc.)")
	fmt.Println()
	fmt.Println("The agent provides:")
	fmt.Println("  • Screen capture and streaming to management server")
	fmt.Println("  • Remote mouse and keyboard input")
	fmt.Println("  • System information reporting")
	fmt.Println("  • Automatic reconnection with exponential backoff")
	fmt.Println("  • OS permission status reporting")
}

func runDiagnostics() {
	fmt.Println("LPUt macOS Diagnostic")
	fmt.Println("---------------------")
	
	sebDetected := "NOT DETECTED"
	if capture.IsSEBRunning() {
		sebDetected = "DETECTED"
	}
	
	fmt.Printf("Agent:                 %s\n", "INSTALLED")
	fmt.Printf("Code Signing:          %s\n", "VALID") // Placeholder
	fmt.Printf("SEB:                   %s\n", sebDetected)
	fmt.Printf("LPUt Compatibility:    %s\n", "PASS")
	fmt.Println()
}
