package permission

// CapabilityStatus describes the status of one OS-level capability.
type CapabilityStatus struct {
	Name       string // "screen_capture", "keyboard_input", "mouse_input", "accessibility", "file_access"
	Status     string // "available", "permission_required", "restricted_by_os", "unavailable", "error"
	Permission string // Human-readable description of the permission needed
	HowToGrant string // Instructions for administrator
}

// PermissionManager checks and reports on OS-level permissions required by the agent.
type PermissionManager interface {
	// CheckAll returns the status of all capabilities.
	CheckAll() []CapabilityStatus
	// CheckCapability checks a specific capability.
	CheckCapability(name string) CapabilityStatus
	// RequestPermission attempts to request a permission (may open OS dialogs).
	RequestPermission(name string) error
}
