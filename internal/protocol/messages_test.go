package protocol

import (
	"encoding/json"
	"testing"
)

func TestMessageSerialization(t *testing.T) {
	msg := Message{
		Type: MsgDeviceRegister,
		Payload: DeviceRegistration{
			DeviceID:     "dev-001",
			Hostname:     "test-machine",
			OS:           "darwin",
			Arch:         "arm64",
			OSVersion:    "macOS 15.0",
			AgentVersion: "1.0.0",
			AuthToken:    "secret-token",
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != MsgDeviceRegister {
		t.Fatalf("Expected type %s, got %s", MsgDeviceRegister, decoded.Type)
	}

	// Decode payload
	payloadBytes, _ := json.Marshal(decoded.Payload)
	var reg DeviceRegistration
	json.Unmarshal(payloadBytes, &reg)

	if reg.DeviceID != "dev-001" {
		t.Fatalf("Expected device ID dev-001, got %s", reg.DeviceID)
	}
	if reg.Hostname != "test-machine" {
		t.Fatalf("Expected hostname test-machine, got %s", reg.Hostname)
	}
}

func TestInputEventSerialization(t *testing.T) {
	ev := InputEvent{
		Type:     "mouse_move",
		X:        0.5,
		Y:        0.75,
		Button:   "left",
		DeltaX:   0,
		DeltaY:   -120,
		Key:      "a",
		Code:     "KeyA",
		AltKey:   false,
		CtrlKey:  true,
		ShiftKey: false,
		MetaKey:  false,
		Display:  0,
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded InputEvent
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.X != 0.5 {
		t.Fatalf("Expected X=0.5, got %f", decoded.X)
	}
	if decoded.Y != 0.75 {
		t.Fatalf("Expected Y=0.75, got %f", decoded.Y)
	}
	if decoded.CtrlKey != true {
		t.Fatal("Expected CtrlKey=true")
	}
	if decoded.Code != "KeyA" {
		t.Fatalf("Expected code KeyA, got %s", decoded.Code)
	}
}

func TestCapabilityStatusSerialization(t *testing.T) {
	report := PermissionStatusReport{
		Capabilities: []CapabilityStatus{
			{
				Name:       "screen_capture",
				Status:     "permission_required",
				Permission: "Screen Recording",
				HowToGrant: "System Settings → Privacy & Security → Screen Recording",
			},
			{
				Name:   "keyboard_input",
				Status: "available",
			},
		},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded PermissionStatusReport
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(decoded.Capabilities) != 2 {
		t.Fatalf("Expected 2 capabilities, got %d", len(decoded.Capabilities))
	}
	if decoded.Capabilities[0].Status != "permission_required" {
		t.Fatalf("Expected permission_required, got %s", decoded.Capabilities[0].Status)
	}
	if decoded.Capabilities[1].Status != "available" {
		t.Fatalf("Expected available, got %s", decoded.Capabilities[1].Status)
	}
}

func TestCoordinateNormalization(t *testing.T) {
	// Test that normalized coordinates (0.0-1.0) convert correctly
	tests := []struct {
		normX, normY float64
		screenW, screenH int
		expectedX, expectedY int
	}{
		{0.0, 0.0, 1920, 1080, 0, 0},
		{1.0, 1.0, 1920, 1080, 1920, 1080},
		{0.5, 0.5, 1920, 1080, 960, 540},
		{0.25, 0.75, 2560, 1440, 640, 1080},
		{0.0, 0.0, 3840, 2160, 0, 0},        // 4K
		{0.5, 0.5, 3840, 2160, 1920, 1080},   // 4K center
	}

	for i, tc := range tests {
		gotX := int(tc.normX * float64(tc.screenW))
		gotY := int(tc.normY * float64(tc.screenH))

		if gotX != tc.expectedX || gotY != tc.expectedY {
			t.Errorf("Test %d: norm(%.2f,%.2f) on %dx%d → got (%d,%d), expected (%d,%d)",
				i, tc.normX, tc.normY, tc.screenW, tc.screenH,
				gotX, gotY, tc.expectedX, tc.expectedY)
		}
	}
}

func TestMultiMonitorCoordinateConversion(t *testing.T) {
	// Simulate multi-monitor with offset: second monitor at X=1920
	tests := []struct {
		normX, normY float64
		monitorOffsetX, monitorOffsetY int
		monitorW, monitorH int
		expectedX, expectedY int
	}{
		{0.5, 0.5, 0, 0, 1920, 1080, 960, 540},          // Primary monitor center
		{0.5, 0.5, 1920, 0, 1920, 1080, 2880, 540},       // Secondary monitor center
		{0.0, 0.0, -1920, 0, 1920, 1080, -1920, 0},        // Left monitor with negative offset
		{1.0, 1.0, 1920, 0, 2560, 1440, 4480, 1440},       // Second monitor bottom-right
	}

	for i, tc := range tests {
		absX := int(tc.normX*float64(tc.monitorW)) + tc.monitorOffsetX
		absY := int(tc.normY*float64(tc.monitorH)) + tc.monitorOffsetY

		if absX != tc.expectedX || absY != tc.expectedY {
			t.Errorf("Test %d: got (%d,%d), expected (%d,%d)", i, absX, absY, tc.expectedX, tc.expectedY)
		}
	}
}

func TestQualityControlSerialization(t *testing.T) {
	qc := QualityControl{
		Quality:    "high",
		FPS:        60,
		Resolution: "native",
	}

	data, _ := json.Marshal(qc)
	var decoded QualityControl
	json.Unmarshal(data, &decoded)

	if decoded.Quality != "high" {
		t.Fatalf("Expected quality high, got %s", decoded.Quality)
	}
	if decoded.FPS != 60 {
		t.Fatalf("Expected FPS 60, got %d", decoded.FPS)
	}
}
