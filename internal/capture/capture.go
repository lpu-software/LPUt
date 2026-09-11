package capture

import (
	"image"
	"time"
)

// FrameData contains compressed display pixels.
type FrameData struct {
	JPEGBytes    []byte
	Width        int
	Height       int
	Timestamp    time.Time
	DisplayIndex int
	CursorX      int
	CursorY      int
	CursorVisible bool
	CaptureDurationMS float64
	EncodeDurationMS  float64
}

// DisplayInfo describes a connected display monitor.
type DisplayInfo struct {
	Index       int
	Bounds      image.Rectangle
	ScaleFactor float64
	IsMain      bool
	Width       int
	Height      int
}

// CaptureStatus indicates the capture system state.
type CaptureStatus string

const (
	StatusAvailable          CaptureStatus = "CAPTURE_AVAILABLE"
	StatusPermissionRequired CaptureStatus = "CAPTURE_PERMISSION_REQUIRED"
	StatusRestrictedByOS     CaptureStatus = "CAPTURE_RESTRICTED_BY_OS"
	StatusUnavailable        CaptureStatus = "CAPTURE_TARGET_UNAVAILABLE"
	StatusError              CaptureStatus = "CAPTURE_ERROR"
)

// ScreenCapture is the platform-abstraction interface for screen capture.
type ScreenCapture interface {
	// GetDisplays returns all connected displays.
	GetDisplays() []DisplayInfo
	// CaptureDisplay captures the given display and returns JPEG frame data.
	CaptureDisplay(displayIndex int) (*FrameData, error)
	// Status returns the current capture capability status.
	Status() CaptureStatus
	// Close releases resources.
	Close() error
}

// QualityConfig controls capture quality parameters.
type QualityConfig struct {
	JPEGQuality   int     // 1-100
	MaxWidth      int     // Max output width (0 = native)
	TargetFPS     int     // Target frames per second
	ScaleFactor   float64 // 1.0 = native, 0.5 = half
}

// DefaultQuality returns sensible defaults.
func DefaultQuality() QualityConfig {
	return QualityConfig{
		JPEGQuality: 50,
		MaxWidth:    1920,
		TargetFPS:   30,
		ScaleFactor: 1.0,
	}
}
