//go:build darwin

package input

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>

static void DarwinInitInput() {
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        NSApplicationLoad();
    });
}

static void DarwinMouseMove(double normX, double normY) {
    DarwinInitInput();
    NSScreen *mainScreen = [NSScreen mainScreen];
    if (!mainScreen) return;
    NSRect frame = [mainScreen frame];

    CGPoint pt = CGPointMake(normX * frame.size.width, normY * frame.size.height);
    CGEventRef ev = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, pt, kCGMouseButtonLeft);
    if (ev) {
        CGEventPost(kCGHIDEventTap, ev);
        CFRelease(ev);
    }
}

static void DarwinMouseButton(const char *button, const char *state, double normX, double normY) {
    DarwinInitInput();
    NSScreen *mainScreen = [NSScreen mainScreen];
    if (!mainScreen) return;
    NSRect frame = [mainScreen frame];

    CGPoint pt = CGPointMake(normX * frame.size.width, normY * frame.size.height);
    CGMouseButton btn = kCGMouseButtonLeft;
    CGEventType downType = kCGEventLeftMouseDown;
    CGEventType upType = kCGEventLeftMouseUp;

    if (strcmp(button, "right") == 0) {
        btn = kCGMouseButtonRight;
        downType = kCGEventRightMouseDown;
        upType = kCGEventRightMouseUp;
    } else if (strcmp(button, "middle") == 0) {
        btn = kCGMouseButtonCenter;
        downType = kCGEventOtherMouseDown;
        upType = kCGEventOtherMouseUp;
    }

    if (strcmp(state, "down") == 0) {
        CGEventRef ev = CGEventCreateMouseEvent(NULL, downType, pt, btn);
        if (ev) { CGEventPost(kCGHIDEventTap, ev); CFRelease(ev); }
    } else if (strcmp(state, "up") == 0) {
        CGEventRef ev = CGEventCreateMouseEvent(NULL, upType, pt, btn);
        if (ev) { CGEventPost(kCGHIDEventTap, ev); CFRelease(ev); }
    } else if (strcmp(state, "click") == 0) {
        CGEventRef down = CGEventCreateMouseEvent(NULL, downType, pt, btn);
        CGEventRef up = CGEventCreateMouseEvent(NULL, upType, pt, btn);
        if (down) { CGEventPost(kCGHIDEventTap, down); CFRelease(down); }
        if (up) { CGEventPost(kCGHIDEventTap, up); CFRelease(up); }
    } else if (strcmp(state, "double_click") == 0) {
        CGEventRef d1 = CGEventCreateMouseEvent(NULL, downType, pt, btn);
        CGEventRef u1 = CGEventCreateMouseEvent(NULL, upType, pt, btn);
        CGEventRef d2 = CGEventCreateMouseEvent(NULL, downType, pt, btn);
        CGEventRef u2 = CGEventCreateMouseEvent(NULL, upType, pt, btn);
        if (d1) { CGEventSetIntegerValueField(d1, kCGMouseEventClickState, 1); CGEventPost(kCGHIDEventTap, d1); CFRelease(d1); }
        if (u1) { CGEventSetIntegerValueField(u1, kCGMouseEventClickState, 1); CGEventPost(kCGHIDEventTap, u1); CFRelease(u1); }
        if (d2) { CGEventSetIntegerValueField(d2, kCGMouseEventClickState, 2); CGEventPost(kCGHIDEventTap, d2); CFRelease(d2); }
        if (u2) { CGEventSetIntegerValueField(u2, kCGMouseEventClickState, 2); CGEventPost(kCGHIDEventTap, u2); CFRelease(u2); }
    }
}

static void DarwinScroll(int deltaX, int deltaY) {
    DarwinInitInput();
    // Negate deltaY because macOS scroll direction is inverted from web deltaY
    CGEventRef scroll = CGEventCreateScrollWheelEvent2(NULL, kCGScrollEventUnitPixel, 2, -deltaY, -deltaX, 0);
    if (scroll) {
        CGEventPost(kCGHIDEventTap, scroll);
        CFRelease(scroll);
    }
}

static void DarwinKeyTap(CGKeyCode keyCode, int flags) {
    DarwinInitInput();
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, keyCode, true);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, keyCode, false);
    if (down && up) {
        CGEventFlags cgFlags = 0;
        if (flags & 1) cgFlags |= kCGEventFlagMaskShift;
        if (flags & 2) cgFlags |= kCGEventFlagMaskControl;
        if (flags & 4) cgFlags |= kCGEventFlagMaskAlternate;
        if (flags & 8) cgFlags |= kCGEventFlagMaskCommand;
        CGEventSetFlags(down, cgFlags);
        CGEventSetFlags(up, cgFlags);
        CGEventPost(kCGHIDEventTap, down);
        CGEventPost(kCGHIDEventTap, up);
    }
    if (down) CFRelease(down);
    if (up) CFRelease(up);
}
*/
import "C"
import (
	"unsafe"
)

// DarwinInputController implements InputController on macOS via CoreGraphics events.
type DarwinInputController struct{}

// NewInputController creates a macOS input controller.
func NewInputController() (InputController, error) {
	C.DarwinInitInput()
	return &DarwinInputController{}, nil
}

func (d *DarwinInputController) MoveMouse(x, y float64) error {
	C.DarwinMouseMove(C.double(x), C.double(y))
	return nil
}

func (d *DarwinInputController) MouseButton(button, state string, x, y float64) error {
	cBtn := C.CString(button)
	defer C.free(unsafe.Pointer(cBtn))
	cState := C.CString(state)
	defer C.free(unsafe.Pointer(cState))
	C.DarwinMouseButton(cBtn, cState, C.double(x), C.double(y))
	return nil
}

func (d *DarwinInputController) Scroll(deltaX, deltaY int) error {
	C.DarwinScroll(C.int(deltaX), C.int(deltaY))
	return nil
}

func (d *DarwinInputController) KeyPress(key, code string, mods Modifiers) error {
	keyCode := mapKeyCode(code, key)
	flags := 0
	if mods.Shift { flags |= 1 }
	if mods.Ctrl  { flags |= 2 }
	if mods.Alt   { flags |= 4 }
	if mods.Meta  { flags |= 8 }
	C.DarwinKeyTap(C.CGKeyCode(keyCode), C.int(flags))
	return nil
}

func (d *DarwinInputController) KeyDown(key, code string, mods Modifiers) error {
	return d.KeyPress(key, code, mods)
}

func (d *DarwinInputController) KeyUp(key, code string, mods Modifiers) error {
	return nil // KeyPress already does down+up
}

func (d *DarwinInputController) TypeText(text string) error {
	for _, ch := range text {
		keyCode := charToKeyCode(ch)
		needsShift := charNeedsShift(ch)
		flags := 0
		if needsShift { flags |= 1 }
		C.DarwinKeyTap(C.CGKeyCode(keyCode), C.int(flags))
	}
	return nil
}

func (d *DarwinInputController) Close() error { return nil }

// mapKeyCode maps web key codes to macOS CGKeyCode values.
func mapKeyCode(code, key string) uint16 {
	// Map by code first (more reliable)
	codeMap := map[string]uint16{
		"KeyA": 0, "KeyB": 11, "KeyC": 8, "KeyD": 2, "KeyE": 14, "KeyF": 3,
		"KeyG": 5, "KeyH": 4, "KeyI": 34, "KeyJ": 38, "KeyK": 40, "KeyL": 37,
		"KeyM": 46, "KeyN": 45, "KeyO": 31, "KeyP": 35, "KeyQ": 12, "KeyR": 15,
		"KeyS": 1, "KeyT": 17, "KeyU": 32, "KeyV": 9, "KeyW": 13, "KeyX": 7,
		"KeyY": 16, "KeyZ": 6,
		"Digit0": 29, "Digit1": 18, "Digit2": 19, "Digit3": 20, "Digit4": 21,
		"Digit5": 23, "Digit6": 22, "Digit7": 26, "Digit8": 28, "Digit9": 25,
		"Enter": 36, "Escape": 53, "Backspace": 51, "Tab": 48, "Space": 49,
		"Minus": 27, "Equal": 24, "BracketLeft": 33, "BracketRight": 30,
		"Backslash": 42, "Semicolon": 41, "Quote": 39, "Backquote": 50,
		"Comma": 43, "Period": 47, "Slash": 44,
		"F1": 122, "F2": 120, "F3": 99, "F4": 118, "F5": 96, "F6": 97,
		"F7": 98, "F8": 100, "F9": 101, "F10": 109, "F11": 103, "F12": 111,
		"ArrowUp": 126, "ArrowDown": 125, "ArrowLeft": 123, "ArrowRight": 124,
		"Delete": 117, "Home": 115, "End": 119, "PageUp": 116, "PageDown": 121,
		"CapsLock": 57, "ShiftLeft": 56, "ShiftRight": 60,
		"ControlLeft": 59, "ControlRight": 62,
		"AltLeft": 58, "AltRight": 61,
		"MetaLeft": 55, "MetaRight": 54,
	}
	if v, ok := codeMap[code]; ok {
		return v
	}
	// Fallback by key name
	keyMap := map[string]uint16{
		"Enter": 36, "Escape": 53, "Backspace": 51, "Tab": 48, " ": 49,
		"ArrowUp": 126, "ArrowDown": 125, "ArrowLeft": 123, "ArrowRight": 124,
		"Delete": 117, "Home": 115, "End": 119, "PageUp": 116, "PageDown": 121,
	}
	if v, ok := keyMap[key]; ok {
		return v
	}
	return 0
}

func charToKeyCode(ch rune) uint16 {
	if ch >= 'a' && ch <= 'z' {
		return mapKeyCode("Key"+string(rune('A'+ch-'a')), "")
	}
	if ch >= 'A' && ch <= 'Z' {
		return mapKeyCode("Key"+string(ch), "")
	}
	if ch >= '0' && ch <= '9' {
		return mapKeyCode("Digit"+string(ch), "")
	}
	switch ch {
	case ' ': return 49
	case '\n': return 36
	case '\t': return 48
	default: return 0
	}
}

func charNeedsShift(ch rune) bool {
	return (ch >= 'A' && ch <= 'Z') || ch == '!' || ch == '@' || ch == '#' ||
		ch == '$' || ch == '%' || ch == '^' || ch == '&' || ch == '*' ||
		ch == '(' || ch == ')' || ch == '_' || ch == '+' || ch == '{' ||
		ch == '}' || ch == '|' || ch == ':' || ch == '"' || ch == '<' ||
		ch == '>' || ch == '?' || ch == '~'
}
