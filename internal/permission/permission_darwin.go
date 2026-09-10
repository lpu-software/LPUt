//go:build darwin

package permission

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework CoreGraphics -framework Cocoa -framework ApplicationServices

#import <CoreGraphics/CoreGraphics.h>
#import <Cocoa/Cocoa.h>

static int CheckScreenRecordingPermission() {
    if (@available(macOS 10.15, *)) {
        return CGPreflightScreenCaptureAccess() ? 1 : 0;
    }
    return 1;
}

static int RequestScreenRecordingPermission() {
    if (@available(macOS 10.15, *)) {
        return CGRequestScreenCaptureAccess() ? 1 : 0;
    }
    return 1;
}

static int CheckAccessibilityPermission() {
    NSDictionary *opts = @{(__bridge NSString *)kAXTrustedCheckOptionPrompt: @NO};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)opts) ? 1 : 0;
}

static int RequestAccessibilityPermission() {
    NSDictionary *opts = @{(__bridge NSString *)kAXTrustedCheckOptionPrompt: @YES};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)opts) ? 1 : 0;
}
*/
import "C"
import "os/exec"

// DarwinPermissionManager implements PermissionManager for macOS.
type DarwinPermissionManager struct{}

func NewPermissionManager() PermissionManager {
	return &DarwinPermissionManager{}
}

func (d *DarwinPermissionManager) CheckAll() []CapabilityStatus {
	return []CapabilityStatus{
		d.CheckCapability("screen_capture"),
		d.CheckCapability("accessibility"),
		d.CheckCapability("keyboard_input"),
		d.CheckCapability("mouse_input"),
	}
}

func (d *DarwinPermissionManager) CheckCapability(name string) CapabilityStatus {
	switch name {
	case "screen_capture":
		if C.CheckScreenRecordingPermission() == 1 {
			return CapabilityStatus{
				Name:   "screen_capture",
				Status: "available",
			}
		}
		return CapabilityStatus{
			Name:       "screen_capture",
			Status:     "permission_required",
			Permission: "Screen Recording",
			HowToGrant: "System Settings → Privacy & Security → Screen Recording → Enable LPUt",
		}

	case "accessibility", "keyboard_input", "mouse_input":
		if C.CheckAccessibilityPermission() == 1 {
			return CapabilityStatus{
				Name:   name,
				Status: "available",
			}
		}
		return CapabilityStatus{
			Name:       name,
			Status:     "permission_required",
			Permission: "Accessibility",
			HowToGrant: "System Settings → Privacy & Security → Accessibility → Enable LPUt",
		}

	default:
		return CapabilityStatus{
			Name:   name,
			Status: "unavailable",
		}
	}
}

func (d *DarwinPermissionManager) RequestPermission(name string) error {
	switch name {
	case "screen_capture":
		C.RequestScreenRecordingPermission()
		// Also open System Settings to the right pane
		_ = exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture").Start()
	case "accessibility", "keyboard_input", "mouse_input":
		C.RequestAccessibilityPermission()
		_ = exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Start()
	}
	return nil
}
