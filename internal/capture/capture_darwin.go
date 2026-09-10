//go:build darwin

package capture

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>

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
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
)

// DarwinCapture implements ScreenCapture on macOS using CoreGraphics.
type DarwinCapture struct {
	mu       sync.RWMutex
	displays []DisplayInfo
	quality  QualityConfig
	stopChan chan struct{}
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

	n := screenshot.NumActiveDisplays()
	e.displays = make([]DisplayInfo, 0, n)

	for i := 0; i < n; i++ {
		b := screenshot.GetDisplayBounds(i)
		e.displays = append(e.displays, DisplayInfo{
			Index:       i,
			Bounds:      b,
			ScaleFactor: 2.0, // Retina default; TODO: detect actual scale
			IsMain:      i == 0,
			Width:       b.Dx(),
			Height:      b.Dy(),
		})
	}
}

func (e *DarwinCapture) GetDisplays() []DisplayInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]DisplayInfo, len(e.displays))
	copy(result, e.displays)
	return result
}

func (e *DarwinCapture) Status() CaptureStatus {
	if C.HasScreenCaptureAccess() == 1 {
		return StatusAvailable
	}
	return StatusPermissionRequired
}

func (e *DarwinCapture) CaptureDisplay(displayIndex int) (*FrameData, error) {
	start := time.Now()

	// Check permission before capture to avoid TCC prompt cascade
	if C.HasScreenCaptureAccess() == 0 {
		return nil, fmt.Errorf("screen capture permission denied by macOS TCC")
	}

	img, err := screenshot.CaptureDisplay(displayIndex)
	if err != nil {
		return nil, fmt.Errorf("native screen capture failed: %w", err)
	}

	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	// Get cursor position
	var curX, curY C.double
	C.GetCursorPosition(&curX, &curY)

	// Retina/HiDPI downscaling
	var targetImg image.Image = img
	maxW := e.quality.MaxWidth
	if maxW > 0 && w > maxW {
		targetH := int(float64(h) * (float64(maxW) / float64(w)))
		targetImg = scaleImage(img, maxW, targetH)
		w = maxW
		h = targetH
	}

	var buf bytes.Buffer
	q := e.quality.JPEGQuality
	if q <= 0 {
		q = 50
	}
	if err := jpeg.Encode(&buf, targetImg, &jpeg.Options{Quality: q}); err != nil {
		return nil, fmt.Errorf("jpeg compression failed: %w", err)
	}

	return &FrameData{
		JPEGBytes:     buf.Bytes(),
		Width:         w,
		Height:        h,
		Timestamp:     start,
		DisplayIndex:  displayIndex,
		CursorX:       int(curX),
		CursorY:       int(curY),
		CursorVisible: true,
	}, nil
}

func (e *DarwinCapture) Close() error {
	select {
	case <-e.stopChan:
	default:
		close(e.stopChan)
	}
	return nil
}

// scaleImage performs nearest-neighbor downscale.
func scaleImage(src image.Image, targetW, targetH int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	for y := 0; y < targetH; y++ {
		sy := (y * srcH) / targetH
		for x := 0; x < targetW; x++ {
			sx := (x * srcW) / targetW
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
