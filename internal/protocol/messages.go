package protocol

import "time"

// MessageType constants for all wire protocol messages.
const (
	// Agent → Server
	MsgDeviceRegister   = "device_register"
	MsgHeartbeat        = "heartbeat"
	MsgPermissionStatus = "permission_status"
	MsgSystemInfo       = "system_info"
	MsgScreenFrame      = "screen_frame"

	// Server → Agent
	MsgDeviceRegistered  = "device_registered"
	MsgSessionStart      = "session_start"
	MsgSessionEnd        = "session_end"
	MsgInputEvent        = "input_event"
	MsgClipboardUpdate   = "clipboard_update"
	MsgClipboardRequest  = "clipboard_request"
	MsgQualityControl    = "quality_control"
	MsgSysInfoRequest    = "system_info_request"
	MsgFileTransferStart = "file_transfer_start"
	MsgFileTransferChunk = "file_transfer_chunk"
	MsgFileTransferEnd   = "file_transfer_end"
	MsgCursorPosition    = "cursor_position"

	// Operator/Console → Server
	MsgAuthRequest   = "auth_request"
	MsgConnectDevice = "connect_device"
	MsgDisconnect    = "disconnect"
	MsgListDevices   = "list_devices"

	// Server → Operator/Console
	MsgAuthResponse      = "auth_response"
	MsgSessionEstablished = "session_established"
	MsgDeviceList        = "device_list"
	MsgError             = "error"
)

// Message is the top-level wire protocol envelope.
type Message struct {
	Type      string      `json:"type"`
	ID        string      `json:"id,omitempty"`
	Timestamp int64       `json:"ts,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// DeviceRegistration is sent by the agent to register with the server.
type DeviceRegistration struct {
	DeviceID    string            `json:"device_id"`
	Hostname    string            `json:"hostname"`
	OS          string            `json:"os"`
	Arch        string            `json:"arch"`
	OSVersion   string            `json:"os_version"`
	AgentVersion string           `json:"agent_version"`
	AuthToken   string            `json:"auth_token"`
	Permissions map[string]string `json:"permissions,omitempty"` // capability → status
}

// DeviceRegistered is the server's response to a successful registration.
type DeviceRegistered struct {
	SessionToken string `json:"session_token"`
	ServerTime   int64  `json:"server_time"`
}

// AuthRequest is sent by an operator to authenticate.
type AuthRequest struct {
	Username string `json:"username"`
	APIKey   string `json:"api_key"`
}

// AuthResponse is the server's response to authentication.
type AuthResponse struct {
	Success      bool   `json:"success"`
	OperatorID   string `json:"operator_id,omitempty"`
	SessionToken string `json:"session_token,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
}

// ConnectDeviceRequest is sent by an operator to start a remote session.
type ConnectDeviceRequest struct {
	DeviceID string `json:"device_id"`
	Quality  string `json:"quality,omitempty"` // "low", "medium", "high"
	FPS      int    `json:"fps,omitempty"`     // 15, 30, 60
}

// SessionEstablished is sent to the operator when a remote session is ready.
type SessionEstablished struct {
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id"`
	Hostname  string `json:"hostname"`
}

// InputEvent represents mouse/keyboard/scroll from the remote operator.
type InputEvent struct {
	Type     string  `json:"type"`      // "mouse_move", "mouse_down", "mouse_up", "mouse_click", "double_click", "mouse_scroll", "key_down", "key_up", "key_press"
	X        float64 `json:"x"`         // Normalized 0.0–1.0
	Y        float64 `json:"y"`         // Normalized 0.0–1.0
	Button   string  `json:"button"`    // "left", "right", "middle"
	DeltaX   int     `json:"delta_x"`
	DeltaY   int     `json:"delta_y"`
	Key      string  `json:"key"`       // e.g. "a", "Enter", "F1"
	Code     string  `json:"code"`      // e.g. "KeyA", "Enter"
	AltKey   bool    `json:"alt_key"`
	CtrlKey  bool    `json:"ctrl_key"`
	ShiftKey bool    `json:"shift_key"`
	MetaKey  bool    `json:"meta_key"`
	Display  int     `json:"display"`
}

// ClipboardUpdate is sent when clipboard text changes.
type ClipboardUpdate struct {
	Text string `json:"text"`
}

// FileTransfer messages
type FileTransferStart struct {
	TransferID string `json:"transfer_id"`
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	Direction  string `json:"direction"` // "upload" (console->agent) or "download" (agent->console)
}

type FileTransferChunk struct {
	TransferID string `json:"transfer_id"`
	Offset     int64  `json:"offset"`
	Data       []byte `json:"data"` // base64 encoded by encoding/json natively for []byte
}

type FileTransferEnd struct {
	TransferID string `json:"transfer_id"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

type CursorPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// QualityControl adjusts streaming parameters.
type QualityControl struct {
	Quality    string `json:"quality"`    // "low", "medium", "high"
	FPS        int    `json:"fps"`        // 15, 30, 60
	Resolution string `json:"resolution"` // "native", "75", "50"
}

// PermissionStatusReport from agent describing capability availability.
type PermissionStatusReport struct {
	Capabilities []CapabilityStatus `json:"capabilities"`
}

// CapabilityStatus describes one agent capability.
type CapabilityStatus struct {
	Name       string `json:"name"`        // "screen_capture", "keyboard_input", "mouse_input", "accessibility", "file_access"
	Status     string `json:"status"`      // "available", "permission_required", "restricted_by_os", "unavailable", "error"
	Permission string `json:"permission"`  // Human-readable permission needed
	HowToGrant string `json:"how_to_grant"` // Instructions for admin
}

// SystemInfoData contains collected system information from the agent.
type SystemInfoData struct {
	Hostname      string        `json:"hostname"`
	OS            string        `json:"os"`
	OSVersion     string        `json:"os_version"`
	Arch          string        `json:"arch"`
	CPUModel      string        `json:"cpu_model"`
	CPUCores      int           `json:"cpu_cores"`
	MemoryTotalMB uint64        `json:"memory_total_mb"`
	MemoryUsedMB  uint64        `json:"memory_used_mb"`
	Uptime        time.Duration `json:"uptime"`
	CurrentUser   string        `json:"current_user"`
	Processes     []ProcessInfo `json:"processes,omitempty"`
	Displays      []DisplayInfo `json:"displays,omitempty"`
}

// ProcessInfo describes a running process.
type ProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemoryMB   float32 `json:"memory_mb"`
	Status     string  `json:"status"`
}

// DisplayInfo describes a connected display.
type DisplayInfo struct {
	Index       int     `json:"index"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	ScaleFactor float64 `json:"scale_factor"`
	IsMain      bool    `json:"is_main"`
	MinX        int     `json:"min_x"`
	MinY        int     `json:"min_y"`
}

// DeviceInfo is the server-side view of a registered device.
type DeviceInfo struct {
	DeviceID     string    `json:"device_id"`
	Hostname     string    `json:"hostname"`
	OS           string    `json:"os"`
	Arch         string    `json:"arch"`
	OSVersion    string    `json:"os_version"`
	AgentVersion string    `json:"agent_version"`
	Status       string    `json:"status"` // "online", "offline", "busy"
	LastSeen     time.Time `json:"last_seen"`
	CurrentUser  string    `json:"current_user,omitempty"`
	ActiveSession string  `json:"active_session,omitempty"`
}

// CaptureStatus enumerations.
const (
	CaptureAvailable          = "CAPTURE_AVAILABLE"
	CapturePermissionRequired = "CAPTURE_PERMISSION_REQUIRED"
	CaptureRestrictedByOS     = "CAPTURE_RESTRICTED_BY_OS"
	CaptureTargetUnavailable  = "CAPTURE_TARGET_UNAVAILABLE"
	CaptureError              = "CAPTURE_ERROR"
)
