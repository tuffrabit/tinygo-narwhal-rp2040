package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/config"
	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/storage"

	"tinygo.org/x/tinyfs"
)

func newTestHandler(t *testing.T) (*Handler, *storage.Manager) {
	blockDev := tinyfs.NewMemoryDevice(256, 4096, 64)
	mgr, err := storage.New(blockDev, true)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	return NewHandler(mgr), mgr
}

func TestFrameEncodingDecoding(t *testing.T) {
	// Create a frame
	original := &Frame{
		Cmd:     CmdGetDeviceConfig,
		Payload: []byte{1, 2, 3, 4},
	}

	// Write to buffer
	var buf bytes.Buffer
	if err := WriteFrame(&buf, original); err != nil {
		t.Fatalf("WriteFrame failed: %v", err)
	}

	// Read back
	decoded, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	// Verify
	if decoded.Cmd != original.Cmd {
		t.Errorf("Cmd: expected 0x%x, got 0x%x", original.Cmd, decoded.Cmd)
	}
	if !bytes.Equal(decoded.Payload, original.Payload) {
		t.Errorf("Payload: expected %v, got %v", original.Payload, decoded.Payload)
	}
}

func TestPingCommand(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	frame := &Frame{
		Cmd:     CmdPing,
		Payload: []byte{0xAA, 0xBB, 0xCC},
	}

	resp := handler.Handle(frame)

	if resp.Status != StatusOK {
		t.Errorf("Expected status OK, got 0x%x", resp.Status)
	}
	if !bytes.Equal(resp.Payload, frame.Payload) {
		t.Errorf("Expected echo payload, got %v", resp.Payload)
	}
}

func TestGetSetDeviceConfig(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	// Set device config
	deviceCfg := config.DeviceConfig{
		Flags:         0xDEADBEEF,
		ActiveProfile: 3,
		Brightness:    200,
		DebounceMs:    10,
	}
	data, _ := deviceCfg.MarshalBinary()

	setFrame := &Frame{
		Cmd:     CmdSetDeviceConfig,
		Payload: data,
	}

	setResp := handler.Handle(setFrame)
	if setResp.Status != StatusOK {
		t.Fatalf("SetDeviceConfig failed: status 0x%x", setResp.Status)
	}

	// Get device config back
	getFrame := &Frame{
		Cmd:     CmdGetDeviceConfig,
		Payload: nil,
	}

	getResp := handler.Handle(getFrame)
	if getResp.Status != StatusOK {
		t.Fatalf("GetDeviceConfig failed: status 0x%x", getResp.Status)
	}

	// Verify
	var loaded config.DeviceConfig
	if err := loaded.UnmarshalBinary(getResp.Payload); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if loaded.ActiveProfile != deviceCfg.ActiveProfile {
		t.Errorf("ActiveProfile: expected %d, got %d", deviceCfg.ActiveProfile, loaded.ActiveProfile)
	}
	if loaded.Brightness != deviceCfg.Brightness {
		t.Errorf("Brightness: expected %d, got %d", deviceCfg.Brightness, loaded.Brightness)
	}
}

func TestGetSetProfile(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	// Set profile
	slot := uint8(5)
	profile := config.Profile{
		Version:      config.CurrentVersion, // Must match current version
		Flags:        0x12345678,
		RGBColor:     0x00FF00,
		RGBPattern:   2,
		BindingCount: 1,
	}
	profile.SetName("TestProfile")
	profile.Bindings[0] = config.KeyBinding{
		InputType:   config.BindingTypeKey,
		InputID:     0,
		OutputType:  config.OutputTypeKeyboard,
		OutputValue: 0x04,
		Modifiers:   0,
		Flags:       0,
	}

	profileData, _ := profile.MarshalBinary()
	payload := append([]byte{slot}, profileData...)

	setFrame := &Frame{
		Cmd:     CmdSetProfile,
		Payload: payload,
	}

	setResp := handler.Handle(setFrame)
	if setResp.Status != StatusOK {
		t.Fatalf("SetProfile failed: status 0x%x", setResp.Status)
	}

	// Get profile back
	getFrame := &Frame{
		Cmd:     CmdGetProfile,
		Payload: []byte{slot},
	}

	getResp := handler.Handle(getFrame)
	if getResp.Status != StatusOK {
		t.Fatalf("GetProfile failed: status 0x%x", getResp.Status)
	}

	// Verify
	var loaded config.Profile
	if err := loaded.UnmarshalBinary(getResp.Payload); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if loaded.GetName() != "TestProfile" {
		t.Errorf("Name: expected 'TestProfile', got '%s'", loaded.GetName())
	}
	if loaded.RGBColor != profile.RGBColor {
		t.Errorf("RGBColor: expected 0x%x, got 0x%x", profile.RGBColor, loaded.RGBColor)
	}
}

func TestDeleteProfile(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	// Create profile first
	profile := config.Profile{Version: config.CurrentVersion}
	profile.SetName("ToDelete")
	profileData, _ := profile.MarshalBinary()
	payload := append([]byte{7}, profileData...)

	resp := handler.Handle(&Frame{Cmd: CmdSetProfile, Payload: payload})
	if resp.Status != StatusOK {
		t.Fatalf("Failed to create profile: status 0x%x", resp.Status)
	}

	// Delete it
	delFrame := &Frame{
		Cmd:     CmdDeleteProfile,
		Payload: []byte{7},
	}

	delResp := handler.Handle(delFrame)
	if delResp.Status != StatusOK {
		t.Errorf("DeleteProfile failed: status 0x%x", delResp.Status)
	}

	// Verify it's gone
	getFrame := &Frame{
		Cmd:     CmdGetProfile,
		Payload: []byte{7},
	}

	getResp := handler.Handle(getFrame)
	if getResp.Status != StatusNotFound {
		t.Errorf("Expected StatusNotFound, got 0x%x", getResp.Status)
	}
}

func TestListProfiles(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	// Create some profiles
	for _, slot := range []uint8{0, 2, 5, 10} {
		profile := config.Profile{Version: config.CurrentVersion}
		profile.SetName("Profile")
		data, _ := profile.MarshalBinary()
		payload := append([]byte{slot}, data...)
		resp := handler.Handle(&Frame{Cmd: CmdSetProfile, Payload: payload})
		if resp.Status != StatusOK {
			t.Fatalf("Failed to create profile %d: status 0x%x", slot, resp.Status)
		}
	}

	// List profiles
	listFrame := &Frame{
		Cmd:     CmdListProfiles,
		Payload: nil,
	}

	listResp := handler.Handle(listFrame)
	if listResp.Status != StatusOK {
		t.Fatalf("ListProfiles failed: status 0x%x", listResp.Status)
	}

	// Verify response format: [Count:1][Slot1:1][Slot2:1]...
	if len(listResp.Payload) < 1 {
		t.Fatal("Empty list response")
	}

	count := listResp.Payload[0]
	if count != 4 {
		t.Errorf("Expected 4 profiles, got %d", count)
	}

	if len(listResp.Payload) != int(1+count) {
		t.Errorf("Expected payload length %d, got %d", 1+count, len(listResp.Payload))
	}
}

func TestStorageStats(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	frame := &Frame{
		Cmd:     CmdGetStorageStats,
		Payload: nil,
	}

	resp := handler.Handle(frame)
	if resp.Status != StatusOK {
		t.Fatalf("GetStorageStats failed: status 0x%x", resp.Status)
	}

	// Verify response format: [Total:4][Used:4][Free:4][ProfileCount:1]
	if len(resp.Payload) != 13 {
		t.Errorf("Expected 13 bytes, got %d", len(resp.Payload))
	}

	total := binary.LittleEndian.Uint32(resp.Payload[0:4])
	used := binary.LittleEndian.Uint32(resp.Payload[4:8])
	free := binary.LittleEndian.Uint32(resp.Payload[8:12])
	profileCount := resp.Payload[12]

	if total == 0 {
		t.Error("Total space should not be zero")
	}
	if used > total {
		t.Errorf("Used space (%d) should not exceed total (%d)", used, total)
	}
	if free > total {
		t.Errorf("Free space (%d) should not exceed total (%d)", free, total)
	}
	if profileCount != 0 {
		t.Errorf("Expected 0 profiles initially, got %d", profileCount)
	}
}

func TestFactoryReset(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	// Create some data
	deviceCfg := config.DeviceConfig{ActiveProfile: 1}
	data, _ := deviceCfg.MarshalBinary()
	handler.Handle(&Frame{Cmd: CmdSetDeviceConfig, Payload: data})

	profile := config.Profile{}
	profile.SetName("Profile")
	profileData, _ := profile.MarshalBinary()
	handler.Handle(&Frame{Cmd: CmdSetProfile, Payload: append([]byte{0}, profileData...)})

	// Factory reset
	resetFrame := &Frame{
		Cmd:     CmdFactoryReset,
		Payload: nil,
	}

	resetResp := handler.Handle(resetFrame)
	if resetResp.Status != StatusOK {
		t.Errorf("FactoryReset failed: status 0x%x", resetResp.Status)
	}

	// Verify profiles are gone
	listResp := handler.Handle(&Frame{Cmd: CmdListProfiles})
	if listResp.Payload[0] != 0 {
		t.Error("Expected 0 profiles after reset")
	}
}

func TestGetVersion(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	frame := &Frame{
		Cmd:     CmdGetVersion,
		Payload: nil,
	}

	resp := handler.Handle(frame)
	if resp.Status != StatusOK {
		t.Fatalf("GetVersion failed: status 0x%x", resp.Status)
	}

	// Verify response format: [FirmwareVersionMajor:1][FirmwareVersionMinor:1][ConfigVersion:2]
	if len(resp.Payload) != 4 {
		t.Errorf("Expected 4 bytes, got %d", len(resp.Payload))
	}

	configVersion := binary.LittleEndian.Uint16(resp.Payload[2:4])
	if configVersion != config.CurrentVersion {
		t.Errorf("Expected config version %d, got %d", config.CurrentVersion, configVersion)
	}
}

func TestInvalidCommand(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	frame := &Frame{
		Cmd:     0xFF, // Invalid command
		Payload: nil,
	}

	resp := handler.Handle(frame)
	if resp.Status != StatusInvalidCmd {
		t.Errorf("Expected StatusInvalidCmd, got 0x%x", resp.Status)
	}
}

func TestInvalidData(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	// Try to set device config with wrong size
	frame := &Frame{
		Cmd:     CmdSetDeviceConfig,
		Payload: []byte{1, 2, 3}, // Too short
	}

	resp := handler.Handle(frame)
	if resp.Status != StatusInvalidData {
		t.Errorf("Expected StatusInvalidData, got 0x%x", resp.Status)
	}
}

func TestCRCMismatch(t *testing.T) {
	// Create a frame with invalid CRC
	buf := &bytes.Buffer{}
	buf.WriteByte(SyncByte)
	buf.WriteByte(CmdPing)
	lenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lenBytes, 0)
	buf.Write(lenBytes)
	// Write wrong CRC
	buf.Write([]byte{0xFF, 0xFF})

	_, err := ReadFrame(buf)
	if err != ErrCRCMismatch {
		t.Errorf("Expected ErrCRCMismatch, got %v", err)
	}
}

func TestInvalidFrame(t *testing.T) {
	// Write wrong sync byte
	buf := &bytes.Buffer{}
	buf.WriteByte(0x55) // Wrong sync

	_, err := ReadFrame(buf)
	if err != ErrInvalidFrame {
		t.Errorf("Expected ErrInvalidFrame, got %v", err)
	}
}

func TestProfileVersionMismatch(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	// Create profile with wrong version
	profile := config.Profile{
		Version: config.CurrentVersion + 1, // Wrong version
	}
	profile.SetName("WrongVersion")

	profileData, _ := profile.MarshalBinary()
	payload := append([]byte{0}, profileData...)

	frame := &Frame{
		Cmd:     CmdSetProfile,
		Payload: payload,
	}

	resp := handler.Handle(frame)
	if resp.Status != StatusVersionMismatch {
		t.Errorf("Expected StatusVersionMismatch, got 0x%x", resp.Status)
	}
}

func TestNotFound(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	// Try to get non-existent profile
	frame := &Frame{
		Cmd:     CmdGetProfile,
		Payload: []byte{99},
	}

	resp := handler.Handle(frame)
	if resp.Status != StatusNotFound {
		t.Errorf("Expected StatusNotFound, got 0x%x", resp.Status)
	}
}

func TestDiscoverCommand(t *testing.T) {
	handler, mgr := newTestHandler(t)
	defer mgr.Close()

	frame := &Frame{
		Cmd:     CmdDiscover,
		Payload: nil,
	}

	resp := handler.Handle(frame)
	if resp.Status != StatusOK {
		t.Fatalf("CmdDiscover failed: status 0x%x", resp.Status)
	}

	// Verify response is "tuffpad"
	expected := "tuffpad"
	if string(resp.Payload) != expected {
		t.Errorf("Expected payload '%s', got '%s'", expected, string(resp.Payload))
	}
}
