package input

// Modifiers represents keyboard modifier state.
type Modifiers struct {
	Alt   bool
	Ctrl  bool
	Shift bool
	Meta  bool // Windows key / Command key
}

// InputController is the platform abstraction for mouse/keyboard input injection.
type InputController interface {
	// MoveMouse moves the cursor to normalized coordinates (0.0–1.0).
	MoveMouse(x, y float64) error
	// MouseButton triggers a mouse button action.
	// button: "left", "right", "middle"
	// state: "down", "up", "click", "double_click"
	MouseButton(button, state string, x, y float64) error
	// Scroll sends a scroll event.
	Scroll(deltaX, deltaY int) error
	// KeyPress sends a key press event.
	KeyPress(key, code string, mods Modifiers) error
	// KeyDown sends a key-down event.
	KeyDown(key, code string, mods Modifiers) error
	// KeyUp sends a key-up event.
	KeyUp(key, code string, mods Modifiers) error
	// TypeText types a string of text.
	TypeText(text string) error
	// Close releases resources.
	Close() error
}
