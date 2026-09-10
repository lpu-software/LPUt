package filetransfer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Session tracks an active file transfer.
type Session struct {
	ID        string
	Filename  string
	Size      int64
	File      *os.File
	BytesRead int64
}

// Manager handles active file transfers.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	baseDir  string
}

// NewManager creates a new file transfer manager writing to the user's Downloads directory.
func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not find home dir: %w", err)
	}

	downloadDir := filepath.Join(home, "Downloads", "LPUt-Transfers")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create downloads dir: %w", err)
	}

	return &Manager{
		sessions: make(map[string]*Session),
		baseDir:  downloadDir,
	}, nil
}

// StartUpload initializes a new upload session from the operator.
func (m *Manager) StartUpload(transferID, filename string, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[transferID]; exists {
		return fmt.Errorf("transfer already in progress")
	}

	// Clean filename to prevent path traversal
	safeName := filepath.Base(filename)
	destPath := filepath.Join(m.baseDir, safeName)

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}

	m.sessions[transferID] = &Session{
		ID:       transferID,
		Filename: safeName,
		Size:     size,
		File:     f,
	}

	return nil
}

// WriteChunk writes a chunk of data to an active upload session.
func (m *Manager) WriteChunk(transferID string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, exists := m.sessions[transferID]
	if !exists {
		return fmt.Errorf("transfer session not found")
	}

	n, err := sess.File.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	sess.BytesRead += int64(n)
	return nil
}

// EndUpload finalizes an upload session.
func (m *Manager) EndUpload(transferID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, exists := m.sessions[transferID]
	if !exists {
		return fmt.Errorf("transfer session not found")
	}

	err := sess.File.Close()
	delete(m.sessions, transferID)

	if err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	return nil
}

// ReadChunk reads a chunk of data for download (Agent -> Operator).
// Not implemented in this basic version, but placeholder for full bidirectional transfer.
func (m *Manager) ReadChunk(transferID string, offset int64, size int) ([]byte, error) {
	return nil, io.EOF
}
