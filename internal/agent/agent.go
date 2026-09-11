package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/yatishydv/lput/internal/capture"
	"github.com/yatishydv/lput/internal/clipboard"
	"github.com/yatishydv/lput/internal/filetransfer"
	"github.com/yatishydv/lput/internal/input"
	"github.com/yatishydv/lput/internal/permission"
	"github.com/yatishydv/lput/internal/protocol"
	"github.com/yatishydv/lput/internal/sysinfo"
)

const (
	agentVersion = "1.0.0"
)

// Config holds agent configuration.
type Config struct {
	ServerURL string
	DeviceID  string
	AuthToken string
}

// Agent is the LPUt endpoint agent.
type Agent struct {
	config       Config
	conn         *websocket.Conn
	connMu       sync.Mutex
	screen       capture.ScreenCapture
	inputCtrl    input.InputController
	permMgr      permission.PermissionManager
	ftManager    *filetransfer.Manager
	streaming    bool
	streamMu     sync.Mutex
	stopStream   chan struct{}
	frameChan    chan []byte
	quality      protocol.QualityControl
	done         chan struct{}
}

// New creates a new agent.
func New(cfg Config) *Agent {
	if cfg.DeviceID == "" {
		cfg.DeviceID = getOrCreateDeviceID()
	}

	ftMgr, err := filetransfer.NewManager()
	if err != nil {
		log.Printf("[Agent] Warning: file transfer disabled: %v", err)
	}

	return &Agent{
		config:    cfg,
		ftManager: ftMgr,
		quality: protocol.QualityControl{
			Quality: "medium",
			FPS:     30,
		},
		done: make(chan struct{}),
	}
}

// Run starts the agent and maintains connection with exponential backoff reconnection.
func (a *Agent) Run() error {
	log.Printf("[LPUt Agent] Starting (device=%s, server=%s)", a.config.DeviceID, a.config.ServerURL)

	// Initialize platform components
	var err error
	a.screen, err = capture.NewScreenCapture()
	if err != nil {
		log.Printf("[Agent] WARNING: Screen capture init failed: %v", err)
	}

	a.inputCtrl, err = input.NewInputController()
	if err != nil {
		log.Printf("[Agent] WARNING: Input controller init failed: %v", err)
	}

	a.permMgr = permission.NewPermissionManager()

	// Log permission status
	perms := a.permMgr.CheckAll()
	for _, p := range perms {
		if p.Status != "available" {
			log.Printf("[Agent] PERMISSION: %s → %s (%s)", p.Name, p.Status, p.HowToGrant)
		} else {
			log.Printf("[Agent] PERMISSION: %s → available", p.Name)
		}
	}

	// Connect with exponential backoff
	a.connectLoop()
	return nil
}

func (a *Agent) connectLoop() {
	backoff := time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-a.done:
			return
		default:
		}

		err := a.connect()
		if err != nil {
			log.Printf("[Agent] Connection failed: %v. Retrying in %v...", err, backoff)
			// Add jitter: ±25%
			jitter := time.Duration(float64(backoff) * (0.75 + 0.5*rand.Float64()))
			select {
			case <-time.After(jitter):
			case <-a.done:
				return
			}

			// Exponential backoff: 1s → 2s → 4s → 8s → 16s → 30s → 60s
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		} else {
			// Reset backoff on successful connection
			backoff = time.Second
		}
	}
}

func (a *Agent) connect() error {
	u, err := url.Parse(a.config.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	// Ensure ws:// or wss://
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	if u.Path == "" {
		u.Path = "/ws/agent"
	}

	log.Printf("[Agent] Connecting to %s...", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	a.connMu.Lock()
	a.conn = conn
	a.connMu.Unlock()

	log.Printf("[Agent] Connected successfully")

	// Register device
	hostname, _ := os.Hostname()
	osVersion := runtime.GOOS + "/" + runtime.GOARCH

	permMap := make(map[string]string)
	for _, p := range a.permMgr.CheckAll() {
		permMap[p.Name] = p.Status
	}

	a.sendJSON(protocol.Message{
		Type: protocol.MsgDeviceRegister,
		Payload: protocol.DeviceRegistration{
			DeviceID:     a.config.DeviceID,
			Hostname:     hostname,
			OS:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			OSVersion:    osVersion,
			AgentVersion: agentVersion,
			AuthToken:    a.config.AuthToken,
			Permissions:  permMap,
		},
	})

	// Start heartbeat
	go a.heartbeatLoop()

	// Read messages
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[Agent] Read error: %v", err)
			a.stopStreaming()
			return fmt.Errorf("read error: %w", err)
		}

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[Agent] Invalid message: %v", err)
			continue
		}

		a.handleMessage(msg)
	}
}

func (a *Agent) handleMessage(msg protocol.Message) {
	switch msg.Type {
	case protocol.MsgDeviceRegistered:
		log.Printf("[Agent] Registration confirmed by server")

	case protocol.MsgSessionStart:
		log.Printf("[Agent] Remote session requested")
		// Parse quality settings
		if msg.Payload != nil {
			qData, _ := json.Marshal(msg.Payload)
			var qc protocol.QualityControl
			if json.Unmarshal(qData, &qc) == nil {
				if qc.FPS > 0 {
					a.quality = qc
				}
			}
		}
		a.startStreaming()

	case protocol.MsgSessionEnd:
		log.Printf("[Agent] Remote session ended")
		a.stopStreaming()

	case protocol.MsgInputEvent:
		a.handleInput(msg)

	case protocol.MsgQualityControl:
		qData, _ := json.Marshal(msg.Payload)
		var qc protocol.QualityControl
		if json.Unmarshal(qData, &qc) == nil {
			a.quality = qc
			log.Printf("[Agent] Quality changed: %s, FPS: %d", qc.Quality, qc.FPS)
		}

	case protocol.MsgSysInfoRequest:
		a.sendSystemInfo()

	case protocol.MsgClipboardUpdate:
		cData, _ := json.Marshal(msg.Payload)
		var cu protocol.ClipboardUpdate
		if json.Unmarshal(cData, &cu) == nil {
			err := clipboard.Write(cu.Text)
			if err != nil {
				log.Printf("[Agent] Failed to write clipboard: %v", err)
			} else {
				log.Printf("[Agent] Clipboard updated from remote")
			}
		}

	case protocol.MsgClipboardRequest:
		text, err := clipboard.Read()
		if err == nil && text != "" {
			a.sendJSON(protocol.Message{
				Type: protocol.MsgClipboardUpdate,
				Payload: protocol.ClipboardUpdate{
					Text: text,
				},
			})
			log.Printf("[Agent] Sent clipboard text to remote")
		} else if err != nil {
			log.Printf("[Agent] Failed to read clipboard: %v", err)
		}

	case protocol.MsgFileTransferStart:
		if a.ftManager == nil {
			return
		}
		data, _ := json.Marshal(msg.Payload)
		var req protocol.FileTransferStart
		if json.Unmarshal(data, &req) == nil && req.Direction == "upload" {
			err := a.ftManager.StartUpload(req.TransferID, req.Filename, req.Size)
			if err != nil {
				log.Printf("[Agent] File transfer start failed: %v", err)
			}
		}

	case protocol.MsgFileTransferChunk:
		if a.ftManager == nil {
			return
		}
		data, _ := json.Marshal(msg.Payload)
		var req protocol.FileTransferChunk
		if json.Unmarshal(data, &req) == nil {
			err := a.ftManager.WriteChunk(req.TransferID, req.Data)
			if err != nil {
				log.Printf("[Agent] File transfer chunk failed: %v", err)
			}
		}

	case protocol.MsgFileTransferEnd:
		if a.ftManager == nil {
			return
		}
		data, _ := json.Marshal(msg.Payload)
		var req protocol.FileTransferEnd
		if json.Unmarshal(data, &req) == nil {
			err := a.ftManager.EndUpload(req.TransferID)
			if err != nil {
				log.Printf("[Agent] File transfer end failed: %v", err)
			} else {
				log.Printf("[Agent] File transfer %s completed successfully", req.TransferID)
			}
		}
	
	case protocol.MsgAgentShutdown:
		log.Printf("[Agent] Received shutdown command. Self-destructing...")
		os.Exit(0)
	}
}

func (a *Agent) handleInput(msg protocol.Message) {
	if a.inputCtrl == nil {
		return
	}

	evData, _ := json.Marshal(msg.Payload)
	var ev protocol.InputEvent
	if err := json.Unmarshal(evData, &ev); err != nil {
		return
	}

	switch ev.Type {
	case "mouse_move":
		a.inputCtrl.MoveMouse(ev.X, ev.Y)
	case "mouse_down":
		a.inputCtrl.MouseButton(ev.Button, "down", ev.X, ev.Y)
	case "mouse_up":
		a.inputCtrl.MouseButton(ev.Button, "up", ev.X, ev.Y)
	case "mouse_click":
		a.inputCtrl.MouseButton(ev.Button, "click", ev.X, ev.Y)
	case "double_click":
		btn := ev.Button
		if btn == "" { btn = "left" }
		a.inputCtrl.MouseButton(btn, "double_click", ev.X, ev.Y)
	case "mouse_scroll":
		a.inputCtrl.Scroll(ev.DeltaX, ev.DeltaY)
	case "key_press":
		a.inputCtrl.KeyPress(ev.Key, ev.Code, input.Modifiers{
			Alt: ev.AltKey, Ctrl: ev.CtrlKey, Shift: ev.ShiftKey, Meta: ev.MetaKey,
		})
	case "key_down":
		a.inputCtrl.KeyDown(ev.Key, ev.Code, input.Modifiers{
			Alt: ev.AltKey, Ctrl: ev.CtrlKey, Shift: ev.ShiftKey, Meta: ev.MetaKey,
		})
	case "key_up":
		a.inputCtrl.KeyUp(ev.Key, ev.Code, input.Modifiers{
			Alt: ev.AltKey, Ctrl: ev.CtrlKey, Shift: ev.ShiftKey, Meta: ev.MetaKey,
		})
	}
}

func (a *Agent) startStreaming() {
	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	if a.streaming {
		return
	}
	if a.screen == nil {
		log.Printf("[Agent] Cannot start streaming: screen capture not available")
		return
	}

	status := a.screen.Status()
	if status != capture.StatusAvailable {
		log.Printf("[Agent] Cannot start streaming: capture status = %s", status)
		return
	}

	a.streaming = true
	a.stopStream = make(chan struct{})
	a.frameChan = make(chan []byte, 1)
	go a.trackCursor()
	log.Printf("[Agent] Streaming started (FPS=%d, Quality=%s)", a.quality.FPS, a.quality.Quality)

	// Start dedicated frame sender goroutine
	go func() {
		for {
			select {
			case <-a.stopStream:
				return
			case frame := <-a.frameChan:
				a.sendBinary(frame)
			}
		}
	}()

	go func() {
		fps := a.quality.FPS
		if fps <= 0 { fps = 30 }
		interval := time.Second / time.Duration(fps)
		frameCount := 0

		for {
			select {
			case <-a.stopStream:
				return
			default:
			}

			start := time.Now()
			frame, err := a.screen.CaptureDisplay(0)
			if err != nil {
				log.Printf("[Agent] Capture error: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if frame != nil && len(frame.JPEGBytes) > 0 {
				// Non-blocking latest-frame drop strategy
				select {
				case a.frameChan <- frame.JPEGBytes:
				default:
					// Drop the old frame and replace with the new one
					select {
					case <-a.frameChan:
					default:
					}
					// Send the latest frame
					select {
					case a.frameChan <- frame.JPEGBytes:
					default:
					}
				}

				// Periodically send performance stats (about once per second)
				if frameCount%fps == 0 {
					stats := protocol.PerformanceStats{
						CaptureLatencyMS: frame.CaptureDurationMS,
						EncodeLatencyMS:  frame.EncodeDurationMS,
						FPS:              float64(fps),
						Resolution:       fmt.Sprintf("%dx%d", frame.Width, frame.Height),
					}
					a.sendJSON(protocol.Message{
						Type:    protocol.MsgPerformanceStats,
						Payload: stats,
					})
				}
				frameCount++
			}

			elapsed := time.Since(start)
			if elapsed < interval {
				time.Sleep(interval - elapsed)
			}
		}
	}()
}

func (a *Agent) stopStreaming() {
	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	if !a.streaming {
		return
	}
	a.streaming = false
	close(a.stopStream)
	log.Printf("[Agent] Streaming stopped")
}

func (a *Agent) trackCursor() {
	ticker := time.NewTicker(time.Millisecond * 33) // ~30Hz
	defer ticker.Stop()

	var lastX, lastY int

	for {
		select {
		case <-a.stopStream:
			return
		case <-ticker.C:
			x, y := robotgo.GetMousePos()
			if x != lastX || y != lastY {
				lastX, lastY = x, y
				a.sendJSON(protocol.Message{
					Type: protocol.MsgCursorPosition,
					Payload: protocol.CursorPosition{
						X: x,
						Y: y,
					},
				})
			}
		}
	}
}

func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.sendJSON(protocol.Message{
				Type:      protocol.MsgHeartbeat,
				Timestamp: time.Now().Unix(),
			})
		case <-a.done:
			return
		}
	}
}

func (a *Agent) sendSystemInfo() {
	info, err := sysinfo.Collect()
	if err != nil {
		log.Printf("[Agent] Failed to collect system info: %v", err)
		return
	}

	// Add display info
	if a.screen != nil {
		displays := a.screen.GetDisplays()
		for _, d := range displays {
			info.Displays = append(info.Displays, protocol.DisplayInfo{
				Index:       d.Index,
				Width:       d.Width,
				Height:      d.Height,
				ScaleFactor: d.ScaleFactor,
				IsMain:      d.IsMain,
				MinX:        d.Bounds.Min.X,
				MinY:        d.Bounds.Min.Y,
			})
		}
	}

	a.sendJSON(protocol.Message{
		Type:    protocol.MsgSystemInfo,
		Payload: info,
	})
}

func (a *Agent) sendJSON(v interface{}) {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	if a.conn != nil {
		_ = a.conn.WriteJSON(v)
	}
}

func (a *Agent) sendBinary(data []byte) {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	if a.conn != nil {
		_ = a.conn.WriteMessage(websocket.BinaryMessage, data)
	}
}

// Stop cleanly shuts down the agent.
func (a *Agent) Stop() {
	select {
	case <-a.done:
	default:
		close(a.done)
	}

	a.stopStreaming()

	if a.screen != nil {
		a.screen.Close()
	}
	if a.inputCtrl != nil {
		a.inputCtrl.Close()
	}

	a.connMu.Lock()
	if a.conn != nil {
		a.conn.Close()
	}
	a.connMu.Unlock()
}

func getOrCreateDeviceID() string {
	home, _ := os.UserHomeDir()
	dir := home + "/.lput"
	_ = os.MkdirAll(dir, 0755)
	idPath := dir + "/device-id"

	data, err := os.ReadFile(idPath)
	if err == nil && len(data) > 0 {
		return string(data)
	}

	id := uuid.New().String()
	_ = os.WriteFile(idPath, []byte(id), 0644)
	return id
}
