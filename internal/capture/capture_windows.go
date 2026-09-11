//go:build windows

package capture

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/kbinani/screenshot"
)

const (
	srccopy        = 0x00CC0020
	biRGB          = 0
	dibRGBColors   = 0
	cursorShowing  = 0x00000001
	diNormal       = 0x0003
	diCompat       = 0x0004
)

type point struct{ X, Y int32 }
type cursorinfo struct {
	CbSize      uint32
	Flags       uint32
	HCursor     uintptr
	PtScreenPos point
}
type bitmapinfoheader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}
type bitmapinfo struct {
	BmiHeader bitmapinfoheader
	BmiColors [1]uint32
}

var (
	modUser32              = syscall.NewLazyDLL("user32.dll")
	modGdi32               = syscall.NewLazyDLL("gdi32.dll")
	procGetDC              = modUser32.NewProc("GetDC")
	procReleaseDC          = modUser32.NewProc("ReleaseDC")
	procGetCursorInfo      = modUser32.NewProc("GetCursorInfo")
	procDrawIconEx         = modUser32.NewProc("DrawIconEx")
	procCreateCompatibleDC = modGdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection   = modGdi32.NewProc("CreateDIBSection")
	procSelectObject       = modGdi32.NewProc("SelectObject")
	procBitBlt             = modGdi32.NewProc("BitBlt")
	procDeleteDC           = modGdi32.NewProc("DeleteDC")
	procDeleteObject       = modGdi32.NewProc("DeleteObject")
)

// WindowsCapture implements ScreenCapture on Windows using GDI BitBlt with real cursor compositing.
type WindowsCapture struct {
	mu       sync.RWMutex
	displays []DisplayInfo
	quality  QualityConfig
}

func NewScreenCapture() (ScreenCapture, error) {
	engine := &WindowsCapture{
		quality: DefaultQuality(),
	}
	engine.refreshDisplays()
	return engine, nil
}

func (e *WindowsCapture) refreshDisplays() {
	e.mu.Lock()
	defer e.mu.Unlock()

	n := screenshot.NumActiveDisplays()
	e.displays = make([]DisplayInfo, 0, n)
	for i := 0; i < n; i++ {
		b := screenshot.GetDisplayBounds(i)
		e.displays = append(e.displays, DisplayInfo{
			Index:       i,
			Bounds:      b,
			ScaleFactor: 1.0,
			IsMain:      i == 0,
			Width:       b.Dx(),
			Height:      b.Dy(),
		})
	}
}

func (e *WindowsCapture) GetDisplays() []DisplayInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]DisplayInfo, len(e.displays))
	copy(result, e.displays)
	return result
}

func (e *WindowsCapture) Status() CaptureStatus {
	return StatusAvailable // Windows doesn't need special TCC permissions
}

func (e *WindowsCapture) CaptureDisplay(displayIndex int) (*FrameData, error) {
	start := time.Now()

	bounds := screenshot.GetDisplayBounds(displayIndex)
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		w, h = 1920, 1080
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, fmt.Errorf("GetDC(0) failed")
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hdcMem)

	var bi bitmapinfo
	bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bi.BmiHeader))
	bi.BmiHeader.BiWidth = int32(w)
	bi.BmiHeader.BiHeight = -int32(h) // top-down
	bi.BmiHeader.BiPlanes = 1
	bi.BmiHeader.BiBitCount = 32
	bi.BmiHeader.BiCompression = biRGB

	var pBits unsafe.Pointer
	hBitmap, _, _ := procCreateDIBSection.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&pBits)),
		0, 0,
	)
	if hBitmap == 0 || pBits == nil {
		return nil, fmt.Errorf("CreateDIBSection failed")
	}
	defer procDeleteObject.Call(hBitmap)

	procSelectObject.Call(hdcMem, hBitmap)
	procBitBlt.Call(hdcMem, 0, 0, uintptr(w), uintptr(h), hdcScreen,
		uintptr(bounds.Min.X), uintptr(bounds.Min.Y), srccopy)

	// Composite real system cursor
	var ci cursorinfo
	ci.CbSize = uint32(unsafe.Sizeof(ci))
	cursorX, cursorY := 0, 0
	rCur, _, _ := procGetCursorInfo.Call(uintptr(unsafe.Pointer(&ci)))
	if rCur != 0 && (ci.Flags&cursorShowing) != 0 && ci.HCursor != 0 {
		curX := ci.PtScreenPos.X - int32(bounds.Min.X)
		curY := ci.PtScreenPos.Y - int32(bounds.Min.Y)
		procDrawIconEx.Call(hdcMem, uintptr(curX), uintptr(curY),
			ci.HCursor, 0, 0, 0, 0, diNormal|diCompat)
		cursorX = int(ci.PtScreenPos.X)
		cursorY = int(ci.PtScreenPos.Y)
	}

	// Convert BGRA → RGBA
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	srcSlice := (*[1 << 30]byte)(pBits)[:w*h*4:w*h*4]
	for i := 0; i < len(srcSlice); i += 4 {
		img.Pix[i+0] = srcSlice[i+2] // R
		img.Pix[i+1] = srcSlice[i+1] // G
		img.Pix[i+2] = srcSlice[i+0] // B
		img.Pix[i+3] = 255           // A
	}

	// Downscale if needed
	var targetImg image.Image = img
	maxW := e.quality.MaxWidth
	if maxW > 0 && w > maxW {
		targetH := int(float64(h) * (float64(maxW) / float64(w)))
		targetImg = scaleImageRGBA(img, maxW, targetH)
		w = maxW
		h = targetH
	}

	captureEnd := time.Now()

	var buf bytes.Buffer
	q := e.quality.JPEGQuality
	if q <= 0 {
		q = 45
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
		CursorX:       cursorX,
		CursorY:       cursorY,
		CursorVisible: false, // The real cursor is already embedded in the image
		CaptureDurationMS: float64(captureEnd.Sub(start).Microseconds()) / 1000.0,
		EncodeDurationMS:  float64(time.Since(captureEnd).Microseconds()) / 1000.0,
	}, nil
}

func (e *WindowsCapture) Close() error {
	return nil
}

func scaleImageRGBA(src *image.RGBA, targetW, targetH int) *image.RGBA {
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
