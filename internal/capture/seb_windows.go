//go:build windows

package capture

// IsSEBRunning checks if Safe Exam Browser is currently running.
// On Windows, SEB uses a different mechanism and doesn't trigger the WindowServer false positive,
// so we don't need to intercept screen capture here.
func IsSEBRunning() bool {
	return false
}
