//go:build windows

package input

import (
	"github.com/go-vgo/robotgo"
	"github.com/kbinani/screenshot"
)

// WindowsInputController implements InputController on Windows via robotgo (SendInput).
type WindowsInputController struct{}

func NewInputController() (InputController, error) {
	return &WindowsInputController{}, nil
}

func (w *WindowsInputController) MoveMouse(x, y float64) error {
	bounds := screenshot.GetDisplayBounds(0) // Assume primary display for now
	targetX := float64(bounds.Min.X) + (x * float64(bounds.Dx()))
	targetY := float64(bounds.Min.Y) + (y * float64(bounds.Dy()))
	robotgo.Move(int(targetX), int(targetY))
	return nil
}

func (w *WindowsInputController) MouseButton(button, state string, x, y float64) error {
	bounds := screenshot.GetDisplayBounds(0) // Assume primary display for now
	targetX := float64(bounds.Min.X) + (x * float64(bounds.Dx()))
	targetY := float64(bounds.Min.Y) + (y * float64(bounds.Dy()))
	robotgo.Move(int(targetX), int(targetY))

	btn := button
	if btn == "" {
		btn = "left"
	}

	switch state {
	case "down":
		robotgo.MouseDown(btn)
	case "up":
		robotgo.MouseUp(btn)
	case "click":
		robotgo.Click(btn)
	case "double_click":
		robotgo.Click(btn, true)
	}
	return nil
}

func (w *WindowsInputController) Scroll(deltaX, deltaY int) error {
	if deltaY != 0 {
		robotgo.Scroll(0, deltaY)
	}
	return nil
}

func (w *WindowsInputController) KeyPress(key, code string, mods Modifiers) error {
	k := mapWinKey(key, code)
	if k == "" {
		return nil
	}
	var modKeys []string
	if mods.Ctrl  { modKeys = append(modKeys, "ctrl") }
	if mods.Alt   { modKeys = append(modKeys, "alt") }
	if mods.Shift { modKeys = append(modKeys, "shift") }
	if mods.Meta  { modKeys = append(modKeys, "cmd") }

	if len(modKeys) > 0 {
		robotgo.KeyTap(k, modKeys)
	} else {
		robotgo.KeyTap(k)
	}
	return nil
}

func (w *WindowsInputController) KeyDown(key, code string, mods Modifiers) error {
	return w.KeyPress(key, code, mods)
}

func (w *WindowsInputController) KeyUp(key, code string, mods Modifiers) error {
	return nil
}

func (w *WindowsInputController) TypeText(text string) error {
	robotgo.TypeStr(text)
	return nil
}

func (w *WindowsInputController) Close() error { return nil }

func mapWinKey(key, code string) string {
	// robotgo uses short key names
	codeMap := map[string]string{
		"Enter": "enter", "Escape": "escape", "Backspace": "backspace",
		"Tab": "tab", "Space": "space", "Delete": "delete",
		"ArrowUp": "up", "ArrowDown": "down", "ArrowLeft": "left", "ArrowRight": "right",
		"Home": "home", "End": "end", "PageUp": "pageup", "PageDown": "pagedown",
		"F1": "f1", "F2": "f2", "F3": "f3", "F4": "f4", "F5": "f5", "F6": "f6",
		"F7": "f7", "F8": "f8", "F9": "f9", "F10": "f10", "F11": "f11", "F12": "f12",
		"CapsLock": "capslock",
	}
	if v, ok := codeMap[code]; ok {
		return v
	}
	if v, ok := codeMap[key]; ok {
		return v
	}
	// For single characters, use directly
	if len(key) == 1 {
		return key
	}
	return ""
}
