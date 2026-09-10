//go:build !darwin && !windows

package input

import (
	"github.com/go-vgo/robotgo"
)

// FallbackInputController implements InputController for Linux/other platforms.
type FallbackInputController struct{}

func NewInputController() (InputController, error) {
	return &FallbackInputController{}, nil
}

func (f *FallbackInputController) MoveMouse(x, y float64) error {
	sw, sh := robotgo.GetScreenSize()
	robotgo.Move(int(x*float64(sw)), int(y*float64(sh)))
	return nil
}

func (f *FallbackInputController) MouseButton(button, state string, x, y float64) error {
	sw, sh := robotgo.GetScreenSize()
	robotgo.Move(int(x*float64(sw)), int(y*float64(sh)))
	btn := button
	if btn == "" { btn = "left" }
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

func (f *FallbackInputController) Scroll(deltaX, deltaY int) error {
	if deltaY != 0 {
		robotgo.Scroll(0, deltaY)
	}
	return nil
}

func (f *FallbackInputController) KeyPress(key, code string, mods Modifiers) error {
	if key != "" {
		robotgo.KeyTap(key)
	}
	return nil
}

func (f *FallbackInputController) KeyDown(key, code string, mods Modifiers) error {
	return f.KeyPress(key, code, mods)
}

func (f *FallbackInputController) KeyUp(key, code string, mods Modifiers) error {
	return nil
}

func (f *FallbackInputController) TypeText(text string) error {
	robotgo.TypeStr(text)
	return nil
}

func (f *FallbackInputController) Close() error { return nil }
