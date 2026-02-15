package config

import (
	"bytes"
	"testing"
)

func TestDeviceConfigMarshalUnmarshal(t *testing.T) {
	original := DeviceConfig{
		Version:       1,
		Flags:         0x12345678,
		ActiveProfile: 5,
		Brightness:    128,
		DebounceMs:    10,
		Reserved1:     0,
		Reserved2:     0xABCD,
	}
	
	// Marshal
	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}
	
	if len(data) != 12 {
		t.Errorf("Expected 12 bytes, got %d", len(data))
	}
	
	// Unmarshal
	var decoded DeviceConfig
	if err := decoded.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	
	// Verify
	if decoded.Version != original.Version {
		t.Errorf("Version: expected %d, got %d", original.Version, decoded.Version)
	}
	if decoded.Flags != original.Flags {
		t.Errorf("Flags: expected 0x%x, got 0x%x", original.Flags, decoded.Flags)
	}
	if decoded.ActiveProfile != original.ActiveProfile {
		t.Errorf("ActiveProfile: expected %d, got %d", original.ActiveProfile, decoded.ActiveProfile)
	}
	if decoded.Brightness != original.Brightness {
		t.Errorf("Brightness: expected %d, got %d", original.Brightness, decoded.Brightness)
	}
	if decoded.DebounceMs != original.DebounceMs {
		t.Errorf("DebounceMs: expected %d, got %d", original.DebounceMs, decoded.DebounceMs)
	}
	if decoded.Reserved2 != original.Reserved2 {
		t.Errorf("Reserved2: expected 0x%x, got 0x%x", original.Reserved2, decoded.Reserved2)
	}
}

func TestProfileMarshalUnmarshal(t *testing.T) {
	original := Profile{
		Version:      1,
		Flags:        0xDEADBEEF,
		RGBColor:     0xFF00FF,
		RGBPattern:   3,
		BindingCount: 2,
	}
	original.SetName("Test Profile")
	
	// Add some bindings
	original.Bindings[0] = KeyBinding{
		InputType:   BindingTypeKey,
		InputID:     5,
		OutputType:  OutputTypeKeyboard,
		OutputValue: 0x04, // 'a' key
		Modifiers:   0x02, // Left Shift
		Flags:       0x01, // Tap
	}
	original.Bindings[1] = KeyBinding{
		InputType:   BindingTypeJoystickButton,
		InputID:     0,
		OutputType:  OutputTypeGamepadButton,
		OutputValue: 1 << 0, // Button A
		Modifiers:   0,
		Flags:       0,
	}
	
	// Marshal
	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}
	
	if len(data) != 286 {
		t.Errorf("Expected 286 bytes, got %d", len(data))
	}
	
	// Unmarshal
	var decoded Profile
	if err := decoded.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}
	
	// Verify
	if decoded.Version != original.Version {
		t.Errorf("Version: expected %d, got %d", original.Version, decoded.Version)
	}
	if decoded.Flags != original.Flags {
		t.Errorf("Flags: expected 0x%x, got 0x%x", original.Flags, decoded.Flags)
	}
	if decoded.RGBColor != original.RGBColor {
		t.Errorf("RGBColor: expected 0x%x, got 0x%x", original.RGBColor, decoded.RGBColor)
	}
	if decoded.RGBPattern != original.RGBPattern {
		t.Errorf("RGBPattern: expected %d, got %d", original.RGBPattern, decoded.RGBPattern)
	}
	if decoded.BindingCount != original.BindingCount {
		t.Errorf("BindingCount: expected %d, got %d", original.BindingCount, decoded.BindingCount)
	}
	
	// Check name
	if decoded.GetName() != "Test Profile" {
		t.Errorf("Name: expected 'Test Profile', got '%s'", decoded.GetName())
	}
	
	// Check bindings
	for i := 0; i < 2; i++ {
		if decoded.Bindings[i].InputType != original.Bindings[i].InputType {
			t.Errorf("Bindings[%d].InputType: expected %d, got %d", i, original.Bindings[i].InputType, decoded.Bindings[i].InputType)
		}
		if decoded.Bindings[i].InputID != original.Bindings[i].InputID {
			t.Errorf("Bindings[%d].InputID: expected %d, got %d", i, original.Bindings[i].InputID, decoded.Bindings[i].InputID)
		}
		if decoded.Bindings[i].OutputType != original.Bindings[i].OutputType {
			t.Errorf("Bindings[%d].OutputType: expected %d, got %d", i, original.Bindings[i].OutputType, decoded.Bindings[i].OutputType)
		}
		if decoded.Bindings[i].OutputValue != original.Bindings[i].OutputValue {
			t.Errorf("Bindings[%d].OutputValue: expected 0x%x, got 0x%x", i, original.Bindings[i].OutputValue, decoded.Bindings[i].OutputValue)
		}
		if decoded.Bindings[i].Modifiers != original.Bindings[i].Modifiers {
			t.Errorf("Bindings[%d].Modifiers: expected 0x%x, got 0x%x", i, original.Bindings[i].Modifiers, decoded.Bindings[i].Modifiers)
		}
		if decoded.Bindings[i].Flags != original.Bindings[i].Flags {
			t.Errorf("Bindings[%d].Flags: expected 0x%x, got 0x%x", i, original.Bindings[i].Flags, decoded.Bindings[i].Flags)
		}
	}
}

func TestProfileMarshalWriter(t *testing.T) {
	profile := Profile{
		Version:      1,
		Flags:        0x12345678,
		RGBColor:     0x00FF00,
		RGBPattern:   1,
		BindingCount: 0,
	}
	profile.SetName("Writer Test")
	
	var buf bytes.Buffer
	n, err := profile.Marshal(&buf)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	
	if n != 286 {
		t.Errorf("Expected 286 bytes written, got %d", n)
	}
	
	if buf.Len() != 286 {
		t.Errorf("Expected buffer len 286, got %d", buf.Len())
	}
}

func TestProfileUnmarshalReader(t *testing.T) {
	// Create a profile and marshal it
	original := Profile{
		Version:      42,
		Flags:        0xAABBCCDD,
		RGBColor:     0x112233,
		RGBPattern:   7,
		BindingCount: 0,
	}
	original.SetName("Reader Test")
	
	data, _ := original.MarshalBinary()
	
	// Unmarshal from reader
	var decoded Profile
	if err := decoded.Unmarshal(bytes.NewReader(data)); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	
	if decoded.Version != original.Version {
		t.Errorf("Version mismatch: expected %d, got %d", original.Version, decoded.Version)
	}
	if decoded.GetName() != original.GetName() {
		t.Errorf("Name mismatch: expected '%s', got '%s'", original.GetName(), decoded.GetName())
	}
}

func TestProfileNameHandling(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"Short", "Short"},
		{"ExactlyFifteen!", "ExactlyFifteen!"},        // 15 chars
		{"ThisIsAVeryLongNameThatExceeds", "ThisIsAVeryLong"}, // Truncated to 15
		{"", ""}, // Empty
	}
	
	for _, tt := range tests {
		p := Profile{}
		p.SetName(tt.name)
		
		result := p.GetName()
		if result != tt.expected {
			t.Errorf("SetName('%s'): expected '%s', got '%s'", tt.name, tt.expected, result)
		}
		
		// Verify null termination
		if len(tt.expected) < 15 {
			if p.Name[len(tt.expected)] != 0 {
				t.Errorf("Name '%s' not null-terminated", tt.name)
			}
		}
	}
}

func TestUnmarshalInvalidSize(t *testing.T) {
	var profile Profile
	err := profile.UnmarshalBinary([]byte{1, 2, 3}) // Too short
	if err != ErrInvalidSize {
		t.Errorf("Expected ErrInvalidSize, got %v", err)
	}
	
	var device DeviceConfig
	err = device.UnmarshalBinary([]byte{1, 2}) // Too short
	if err != ErrInvalidSize {
		t.Errorf("Expected ErrInvalidSize, got %v", err)
	}
}

func BenchmarkProfileMarshal(b *testing.B) {
	profile := Profile{
		Version:      1,
		Flags:        0x12345678,
		RGBColor:     0xFF00FF,
		RGBPattern:   3,
		BindingCount: 32,
	}
	profile.SetName("Benchmark Profile")
	
	for i := range profile.Bindings {
		profile.Bindings[i] = KeyBinding{
			InputType:   BindingTypeKey,
			InputID:     uint8(i),
			OutputType:  OutputTypeKeyboard,
			OutputValue: uint16(i),
			Modifiers:   uint8(i % 8),
			Flags:       uint8(i % 4),
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := profile.MarshalBinary()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProfileUnmarshal(b *testing.B) {
	profile := Profile{
		Version:      1,
		Flags:        0x12345678,
		RGBColor:     0xFF00FF,
		RGBPattern:   3,
		BindingCount: 32,
	}
	profile.SetName("Benchmark Profile")
	
	for i := range profile.Bindings {
		profile.Bindings[i] = KeyBinding{
			InputType:   BindingTypeKey,
			InputID:     uint8(i),
			OutputType:  OutputTypeKeyboard,
			OutputValue: uint16(i),
			Modifiers:   uint8(i % 8),
			Flags:       uint8(i % 4),
		}
	}
	
	data, _ := profile.MarshalBinary()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var p Profile
		err := p.UnmarshalBinary(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
