package clipboard

import (
	"fmt"

	"github.com/atotto/clipboard"
)

// Read returns the current text on the system clipboard.
func Read() (string, error) {
	text, err := clipboard.ReadAll()
	if err != nil {
		return "", fmt.Errorf("failed to read clipboard: %w", err)
	}
	return text, nil
}

// Write sets the system clipboard text.
func Write(text string) error {
	err := clipboard.WriteAll(text)
	if err != nil {
		return fmt.Errorf("failed to write clipboard: %w", err)
	}
	return nil
}
