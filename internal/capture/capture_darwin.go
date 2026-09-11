//go:build darwin

package capture

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics -framework ScreenCaptureKit -framework CoreMedia -framework CoreVideo -framework VideoToolbox -framework ImageIO

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#include <stdlib.h>
#include "screencapture_darwin.h"

static void InitAccess() {
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        NSApplicationLoad();
    });
}

static int HasScreenCaptureAccess() {
    if (@available(macOS 10.15, *)) {
        return CGPreflightScreenCaptureAccess() ? 1 : 0;
    }
    return 1;
}

static int RequestScreenCaptureAccess() {
    if (@available(macOS 10.15, *)) {
        return CGRequestScreenCaptureAccess() ? 1 : 0;
    }
    return 1;
}

// Get mouse cursor position from CoreGraphics
static void GetCursorPosition(double *outX, double *outY) {
    CGEventRef event = CGEventCreate(NULL);
    CGPoint cursor = CGEventGetLocation(event);
    *outX = cursor.x;
    *outY = cursor.y;
    CFRelease(event);
}
*/
import "C"
import (
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// DarwinCapture implements ScreenCapture on macOS using ScreenCaptureKit.
type DarwinCapture struct {
	mu           sync.RWMutex
	displays     []DisplayInfo
	quality      QualityConfig
	stopChan     chan struct{}
	sckStarted   bool
	currentFPS   int
	currentMaxW  int
}

// NewScreenCapture creates a macOS screen capture engine.
func NewScreenCapture() (ScreenCapture, error) {
	C.InitAccess()
	engine := &DarwinCapture{
		quality:  DefaultQuality(),
		stopChan: make(chan struct{}),
	}
	engine.refreshDisplays()
	return engine, nil
}

func (e *DarwinCapture) refreshDisplays() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Hardcode for now since we removed kbinani/screenshot
	// In a real app we'd query NSScreen or CGDisplay, but SCShareableContent handles primary bounds internally.
	e.displays = []DisplayInfo{
		{
			Index:       0,
			ScaleFactor: 2.0,
			IsMain:      true,
			Width:       1920,
			Height:      1080,
		},
	}
}

func (e *DarwinCapture) GetDisplays() []DisplayInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]DisplayInfo, len(e.displays))
	copy(result, e.displays)
	return result
}

func (e *DarwinCapture) SetQuality(q QualityConfig) {
	e.mu.Lock()
	e.quality = q
	e.mu.Unlock()
}

func (e *DarwinCapture) Status() CaptureStatus {
	if C.HasScreenCaptureAccess() == 1 {
		return StatusAvailable
	}
	return StatusPermissionRequired
}

func (e *DarwinCapture) RequestAccess() {
	C.RequestScreenCaptureAccess()
}

func (e *DarwinCapture) CaptureDisplay(displayIndex int) (*FrameData, error) {
	start := time.Now()

	if C.HasScreenCaptureAccess() == 0 {
		return nil, fmt.Errorf("screen capture permission denied by macOS TCC")
	}
	
	if IsSEBRunning() {
		return nil, fmt.Errorf("CAPTURE_RESTRICTED: Safe Exam Browser is currently running")
	}
	
	if C.SCK_IsAvailable() == 0 {
		return nil, fmt.Errorf("ScreenCaptureKit is not available on this macOS version")
	}

	e.mu.Lock()
	fps := e.quality.TargetFPS
	maxW := e.quality.MaxWidth
	
	if !e.sckStarted || e.currentFPS != fps || e.currentMaxW != maxW {
		C.SCK_StopCapture()
		res := C.SCK_StartCapture(C.int(displayIndex), C.int(maxW), 0, C.int(fps))
		if res == 0 {
			e.mu.Unlock()
			return nil, fmt.Errorf("failed to start ScreenCaptureKit stream")
		}
		e.sckStarted = true
		e.currentFPS = fps
		e.currentMaxW = maxW
	}
	e.mu.Unlock()

	var length C.int
	ptr := C.SCK_GetLatestFrame(&length)
	
	var curX, curY C.double
	C.GetCursorPosition(&curX, &curY)

	if ptr == nil || length == 0 {
		return nil, nil // No new frame yet
	}
	
	// Convert C pointer to Go slice safely
	jpegBytes := C.GoBytes(unsafe.Pointer(ptr), length)
	C.free(unsafe.Pointer(ptr))

	return &FrameData{
		JPEGBytes:     jpegBytes,
		Width:         maxW,
		Height:        0, // Not explicitly tracked in this fast path
		Timestamp:     start,
		DisplayIndex:  displayIndex,
		CursorX:       int(curX),
		CursorY:       int(curY),
		CursorVisible: false, // OS native cursor is composited by SCK natively
		CaptureDurationMS: float64(time.Since(start).Microseconds()) / 1000.0,
		EncodeDurationMS:  0, // SCK encodes on the GPU directly
	}, nil
}

func (e *DarwinCapture) Close() error {
	select {
	case <-e.stopChan:
	default:
		close(e.stopChan)
		C.SCK_StopCapture()
	}
	return nil
}
