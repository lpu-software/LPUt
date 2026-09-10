package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Log levels
const (
	LevelTrace    = "TRACE"
	LevelDebug    = "DEBUG"
	LevelInfo     = "INFO"
	LevelWarning  = "WARNING"
	LevelError    = "ERROR"
	LevelCritical = "CRITICAL"
)

// AuditEntry is a structured audit log entry.
type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Event     string `json:"event"`
	Message   string `json:"message"`
	DeviceID  string `json:"device_id,omitempty"`
}

// AuditLogger writes structured audit logs.
type AuditLogger struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
}

// NewAuditLogger creates a new audit logger writing to ~/.lput/audit.log
func NewAuditLogger() *AuditLogger {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".lput")
	_ = os.MkdirAll(dir, 0755)

	f, err := os.OpenFile(filepath.Join(dir, "audit.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[Audit] Failed to open audit log: %v", err)
		return &AuditLogger{}
	}

	return &AuditLogger{
		file:    f,
		encoder: json.NewEncoder(f),
	}
}

// Log writes a structured audit entry. Never logs passwords, tokens, or keys.
func (a *AuditLogger) Log(level, event, message, deviceID string) {
	entry := AuditEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Event:     event,
		Message:   message,
		DeviceID:  deviceID,
	}

	// Console output
	log.Printf("[%s] %s: %s", level, event, message)

	// File output
	if a.file != nil && a.encoder != nil {
		a.mu.Lock()
		_ = a.encoder.Encode(entry)
		a.mu.Unlock()
	}
}

// Close closes the audit log file.
func (a *AuditLogger) Close() error {
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}

// ReadAuditLog reads the last N audit entries.
func ReadAuditLog(maxEntries int) ([]AuditEntry, error) {
	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".lput", "audit.log")

	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	var entries []AuditEntry
	decoder := json.NewDecoder(
		struct{ *os.File }{nil},
	)
	// Simple line-by-line parse
	_ = decoder
	lines := splitLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var entry AuditEntry
		if err := json.Unmarshal(line, &entry); err == nil {
			entries = append(entries, entry)
		}
	}

	// Return last N
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	return entries, nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
