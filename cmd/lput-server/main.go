package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/yatishydv/lput/internal/server"
)

func main() {
	port := flag.String("port", "8443", "Server port")
	tlsCert := flag.String("tls-cert", "", "TLS certificate file path")
	tlsKey := flag.String("tls-key", "", "TLS private key file path")
	apiKey := flag.String("api-key", "lput-admin-key", "Operator API key")
	flag.Parse()

	if len(os.Args) > 1 && os.Args[1] == "help" {
		printHelp()
		return
	}

	apiKeys := map[string]string{
		*apiKey: "admin",
	}

	srv := server.New(server.Config{
		Port:    *port,
		TLSCert: *tlsCert,
		TLSKey:  *tlsKey,
		APIKeys: apiKeys,
	})

	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║     LPUt — Enterprise Remote Management     ║")
	fmt.Println("║                  Server v1.0.0               ║")
	fmt.Println("╠══════════════════════════════════════════════╣")
	fmt.Printf("║  Port:      %-33s║\n", *port)
	if *tlsCert != "" {
		fmt.Printf("║  TLS:       %-33s║\n", "Enabled")
	} else {
		fmt.Printf("║  TLS:       %-33s║\n", "Disabled (dev mode)")
	}
	fmt.Printf("║  Console:   %-33s║\n", "http://localhost:"+*port)
	fmt.Println("╚══════════════════════════════════════════════╝")

	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}

func printHelp() {
	fmt.Println("LPUt Server — Enterprise Remote Management")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  lput-server [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --port <port>       Server port (default: 8443)")
	fmt.Println("  --tls-cert <file>   TLS certificate file")
	fmt.Println("  --tls-key <file>    TLS private key file")
	fmt.Println("  --api-key <key>     Operator API key (default: lput-admin-key)")
	fmt.Println()
	fmt.Println("The server provides:")
	fmt.Println("  • Agent registration and management")
	fmt.Println("  • Web management console at /")
	fmt.Println("  • WebSocket control at /ws/agent and /ws/console")
	fmt.Println("  • REST API at /api/devices, /api/sessions, /api/auth")
}
