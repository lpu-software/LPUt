package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yatishydv/lput/internal/protocol"
)

func TestServerAgentRegistration(t *testing.T) {
	srv := New(Config{Port: "0", APIKeys: map[string]string{"test-key": "tester"}})
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/agent", srv.handleAgentWS)
	mux.HandleFunc("/ws/console", srv.handleConsoleWS)
	mux.HandleFunc("/api/devices", srv.handleDevices)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// 1. Agent connects and registers
	agentConn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws/agent", nil)
	if err != nil {
		t.Fatalf("Agent dial failed: %v", err)
	}
	defer agentConn.Close()

	err = agentConn.WriteJSON(protocol.Message{
		Type: protocol.MsgDeviceRegister,
		Payload: protocol.DeviceRegistration{
			DeviceID:     "test-device-001",
			Hostname:     "test-machine",
			OS:           "darwin",
			Arch:         "arm64",
			OSVersion:    "macOS 15.0",
			AgentVersion: "1.0.0",
			AuthToken:    "test-token",
		},
	})
	if err != nil {
		t.Fatalf("Register write failed: %v", err)
	}

	// Read registration response
	var resp protocol.Message
	agentConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	err = agentConn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Register read failed: %v", err)
	}

	if resp.Type != protocol.MsgDeviceRegistered {
		t.Fatalf("Expected device_registered, got: %s", resp.Type)
	}

	t.Logf("Agent registered successfully: %+v", resp)

	// 2. Verify device appears in list
	time.Sleep(100 * time.Millisecond) // Allow registration to settle
	devResp, err := http.Get(ts.URL + "/api/devices")
	if err != nil {
		t.Fatalf("Device list request failed: %v", err)
	}
	defer devResp.Body.Close()

	var devices []protocol.DeviceInfo
	json.NewDecoder(devResp.Body).Decode(&devices)

	if len(devices) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(devices))
	}
	if devices[0].DeviceID != "test-device-001" {
		t.Fatalf("Expected device ID test-device-001, got %s", devices[0].DeviceID)
	}
	if devices[0].Status != "online" {
		t.Fatalf("Expected device status online, got %s", devices[0].Status)
	}

	t.Logf("Device in list: %+v", devices[0])
}

func TestServerConsoleDeviceList(t *testing.T) {
	srv := New(Config{Port: "0", APIKeys: map[string]string{"test-key": "tester"}})
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/agent", srv.handleAgentWS)
	mux.HandleFunc("/ws/console", srv.handleConsoleWS)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Register an agent
	agentConn, _, _ := websocket.DefaultDialer.Dial(wsURL+"/ws/agent", nil)
	defer agentConn.Close()

	agentConn.WriteJSON(protocol.Message{
		Type: protocol.MsgDeviceRegister,
		Payload: protocol.DeviceRegistration{
			DeviceID: "dev-123",
			Hostname: "workstation-A",
			OS:       "darwin",
			Arch:     "arm64",
		},
	})

	// Read registration ack
	agentConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var ack protocol.Message
	agentConn.ReadJSON(&ack)

	// Connect console and request device list
	consoleConn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws/console", nil)
	if err != nil {
		t.Fatalf("Console dial failed: %v", err)
	}
	defer consoleConn.Close()

	err = consoleConn.WriteJSON(protocol.Message{Type: protocol.MsgListDevices})
	if err != nil {
		t.Fatalf("List devices write failed: %v", err)
	}

	consoleConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var listResp protocol.Message
	err = consoleConn.ReadJSON(&listResp)
	if err != nil {
		t.Fatalf("List devices read failed: %v", err)
	}

	if listResp.Type != protocol.MsgDeviceList {
		t.Fatalf("Expected device_list, got: %s", listResp.Type)
	}

	// Decode payload
	payloadBytes, _ := json.Marshal(listResp.Payload)
	var devices []protocol.DeviceInfo
	json.Unmarshal(payloadBytes, &devices)

	if len(devices) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(devices))
	}
	if devices[0].Hostname != "workstation-A" {
		t.Fatalf("Expected hostname workstation-A, got %s", devices[0].Hostname)
	}

	t.Logf("Console received device list: %+v", devices)
}

func TestServerSessionLifecycle(t *testing.T) {
	srv := New(Config{Port: "0", APIKeys: map[string]string{"k": "admin"}})
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/agent", srv.handleAgentWS)
	mux.HandleFunc("/ws/console", srv.handleConsoleWS)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Register agent
	agentConn, _, _ := websocket.DefaultDialer.Dial(wsURL+"/ws/agent", nil)
	defer agentConn.Close()

	agentConn.WriteJSON(protocol.Message{
		Type: protocol.MsgDeviceRegister,
		Payload: protocol.DeviceRegistration{DeviceID: "sess-dev", Hostname: "sess-host", OS: "darwin"},
	})
	agentConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var ack protocol.Message
	agentConn.ReadJSON(&ack)

	// Connect console
	consoleConn, _, _ := websocket.DefaultDialer.Dial(wsURL+"/ws/console", nil)
	defer consoleConn.Close()

	// Request session
	consoleConn.WriteJSON(protocol.Message{
		Type: protocol.MsgConnectDevice,
		Payload: protocol.ConnectDeviceRequest{DeviceID: "sess-dev", Quality: "medium", FPS: 30},
	})

	// Agent should receive session_start
	agentConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var agentMsg protocol.Message
	err := agentConn.ReadJSON(&agentMsg)
	if err != nil {
		t.Fatalf("Agent read session_start failed: %v", err)
	}
	if agentMsg.Type != protocol.MsgSessionStart {
		t.Fatalf("Expected session_start, got: %s", agentMsg.Type)
	}
	t.Logf("Agent received session_start: %s", agentMsg.ID)

	// Console should receive session_established
	consoleConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var consoleMsg protocol.Message
	err = consoleConn.ReadJSON(&consoleMsg)
	if err != nil {
		t.Fatalf("Console read session_established failed: %v", err)
	}
	if consoleMsg.Type != protocol.MsgSessionEstablished {
		t.Fatalf("Expected session_established, got: %s", consoleMsg.Type)
	}

	t.Logf("Session established successfully: %+v", consoleMsg)

	// Disconnect
	consoleConn.WriteJSON(protocol.Message{Type: protocol.MsgDisconnect})

	// Agent should receive session_end
	agentConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var endMsg protocol.Message
	err = agentConn.ReadJSON(&endMsg)
	if err != nil {
		t.Fatalf("Agent read session_end failed: %v", err)
	}
	if endMsg.Type != protocol.MsgSessionEnd {
		t.Fatalf("Expected session_end, got: %s", endMsg.Type)
	}

	t.Logf("Session ended successfully")
}

func TestServerInputForwarding(t *testing.T) {
	srv := New(Config{Port: "0", APIKeys: map[string]string{"k": "admin"}})
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/agent", srv.handleAgentWS)
	mux.HandleFunc("/ws/console", srv.handleConsoleWS)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Setup: agent + console + session
	agentConn, _, _ := websocket.DefaultDialer.Dial(wsURL+"/ws/agent", nil)
	defer agentConn.Close()
	agentConn.WriteJSON(protocol.Message{
		Type:    protocol.MsgDeviceRegister,
		Payload: protocol.DeviceRegistration{DeviceID: "input-dev", Hostname: "input-host"},
	})
	agentConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var ack protocol.Message
	agentConn.ReadJSON(&ack)

	consoleConn, _, _ := websocket.DefaultDialer.Dial(wsURL+"/ws/console", nil)
	defer consoleConn.Close()
	consoleConn.WriteJSON(protocol.Message{
		Type:    protocol.MsgConnectDevice,
		Payload: protocol.ConnectDeviceRequest{DeviceID: "input-dev"},
	})

	// Drain session_start and session_established
	agentConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	agentConn.ReadJSON(&ack)
	consoleConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	consoleConn.ReadJSON(&ack)

	// Send input event from console
	consoleConn.WriteJSON(protocol.Message{
		Type: protocol.MsgInputEvent,
		Payload: protocol.InputEvent{
			Type: "mouse_move",
			X:    0.5,
			Y:    0.5,
		},
	})

	// Agent should receive input event
	agentConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var inputMsg protocol.Message
	err := agentConn.ReadJSON(&inputMsg)
	if err != nil {
		t.Fatalf("Agent read input failed: %v", err)
	}
	if inputMsg.Type != protocol.MsgInputEvent {
		t.Fatalf("Expected input_event, got: %s", inputMsg.Type)
	}

	t.Logf("Input event forwarded successfully: %+v", inputMsg)
}
