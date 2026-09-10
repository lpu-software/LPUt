//go:build !darwin && !windows

package capture

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
)

// FallbackCapture implements ScreenCapture for Linux/other using kbinani/screenshot.
type FallbackCapture struct {
	mu       sync.RWMutex
	displays []DisplayInfo
	quality  QualityConfig
}

func NewScreenCapture() (ScreenCapture, error) {
	engine := &FallbackCapture{
		quality: DefaultQuality(),
	}
	engine.refreshDisplays()
	return engine, nil
}

func (e *FallbackCapture) refreshDisplays() {
	e.mu.Lock()
	defer e.mu.Unlock()

	n := screenshot.NumActiveDisplays()
	e.displays = make([]DisplayInfo, 0, n)
	for i := 0; i < n; i++ {
		b := screenshot.GetDisplayBounds(i)
		e.displays = append(e.displays, DisplayInfo{
			Index:  i,
			Bounds: b,
			ScaleFactor: 1.0,
			IsMain: i == 0,
			Width:  b.Dx(),
			Height: b.Dy(),
		})
	}
}

func (e *FallbackCapture) GetDisplays() []DisplayInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]DisplayInfo, len(e.displays))
	copy(result, e.displays)
	return result
}

func (e *FallbackCapture) Status() CaptureStatus {
	return StatusAvailable
}

func (e *FallbackCapture) CaptureDisplay(displayIndex int) (*FrameData, error) {
	start := time.Now()
	bounds := screenshot.GetDisplayBounds(displayIndex)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("capture failed: %w", err)
	}

	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	var targetImg image.Image = img
	maxW := e.quality.MaxWidth
	if maxW > 0 && w > maxW {
		targetH := int(float64(h) * (float64(maxW) / float64(w)))
		targetImg = scaleImageFallback(img, maxW, targetH)
		w = maxW
		h = targetH
	}

	var buf bytes.Buffer
	q := e.quality.JPEGQuality
	if q <= 0 { q = 45 }
	if err := jpeg.Encode(&buf, targetImg, &jpeg.Options{Quality: q}); err != nil {
		return nil, fmt.Errorf("jpeg encode failed: %w", err)
	}

	return &FrameData{
		JPEGBytes:    buf.Bytes(),
		Width:        w,
		Height:       h,
		Timestamp:    start,
		DisplayIndex: displayIndex,
	}, nil
}

func (e *FallbackCapture) Close() error { return nil }

func scaleImageFallback(src *image.RGBA, targetW, targetH int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	for y := 0; y < targetH; y++ {
		sy := (y * srcH) / targetH
		for x := 0; x < targetW; x++ {
			sx := (x * srcW) / targetW
			dst.SetRGBA(x, y, src.RGBAAt(sx, sy))
		}
	}
	return dst
}
