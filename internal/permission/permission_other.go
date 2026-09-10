//go:build !darwin

package permission

// DefaultPermissionManager for Windows and other platforms where no TCC-style prompts exist.
type DefaultPermissionManager struct{}

func NewPermissionManager() PermissionManager {
	return &DefaultPermissionManager{}
}

func (d *DefaultPermissionManager) CheckAll() []CapabilityStatus {
	return []CapabilityStatus{
		{Name: "screen_capture", Status: "available"},
		{Name: "keyboard_input", Status: "available"},
		{Name: "mouse_input", Status: "available"},
		{Name: "accessibility", Status: "available"},
	}
}

func (d *DefaultPermissionManager) CheckCapability(name string) CapabilityStatus {
	return CapabilityStatus{Name: name, Status: "available"}
}

func (d *DefaultPermissionManager) RequestPermission(name string) error {
	return nil
}
