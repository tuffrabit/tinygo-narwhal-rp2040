// Package config defines the configuration data structures for the gamepad.
// All structs are designed for zero-allocation binary serialization.
package config

import (
	"encoding/binary"
	"errors"
	"io"
)

// CurrentVersion is the config format version.
// Bump this when making breaking changes to the config format.
// When firmware boots and finds a different version in flash, configs are wiped.
const CurrentVersion uint16 = 1

// BindingType indicates what kind of input this binding responds to
type BindingType uint8

const (
	BindingTypeKey BindingType = iota
	BindingTypeJoystickButton
	BindingTypeDPad
	BindingTypeRGBPattern // Special RGB pattern trigger
)

// OutputType indicates what the binding emits
type OutputType uint8

const (
	OutputTypeNone OutputType = iota
	OutputTypeKeyboard
	OutputTypeGamepadButton
	OutputTypeMouseButton
	OutputTypeConsumer // Media keys, etc.
)

// KeyBinding maps one input to one output.
// Total size: 8 bytes
// Packed layout: [InputType:1][InputID:1][OutputType:1][OutputValueHi:1][OutputValueLo:1][Modifiers:1][Flags:1][Reserved:1]
type KeyBinding struct {
	InputType   BindingType // 1 byte
	InputID     uint8       // Which key/button/dpad (0-31)
	OutputType  OutputType  // 1 byte
	OutputValue uint16      // HID keycode or button mask
	Modifiers   uint8       // Ctrl/Shift/Alt/Gui (HID modifier byte)
	Flags       uint8       // Tap/Hold/Double-tap/etc
	Reserved    uint8       // Padding to 8 bytes total
}

// Profile config for one keybinding layer.
// This is a fixed-size struct for zero-allocation binary serialization.
// Total size: 280 bytes
// Layout:
//   [0-1]:   Version (uint16)
//   [2-5]:   Flags (uint32)
//   [6-9]:   RGBColor (uint32)
//   [10]:    RGBPattern (uint8)
//   [11]:    Reserved (uint8)
//   [12]:    BindingCount (uint8)
//   [13]:    Reserved (uint8)
//   [14-29]: Name ([16]byte)
//   [30-285]: Bindings ([32]KeyBinding)
type Profile struct {
	Version      uint16         // Config format version
	Flags        uint32         // Profile-level flags (KB mode enabled, etc.)
	RGBColor     uint32         // RGB LED color (RGB888)
	RGBPattern   uint8          // RGB pattern ID
	Reserved1    uint8          // Padding
	BindingCount uint8          // Actual number of bindings (<= 32)
	Reserved2    uint8          // Padding
	Name         [16]byte       // UTF-8 name (null-terminated if shorter)
	Bindings     [32]KeyBinding // Fixed array, uses BindingCount
}

// Device global settings.
// Total size: 12 bytes
// Layout:
//   [0-1]:  Version (uint16)
//   [2-5]:  Flags (uint32)
//   [6]:    ActiveProfile (uint8)
//   [7]:    Brightness (uint8)
//   [8]:    DebounceMs (uint8)
//   [9]:    Reserved (uint8)
//   [10-11]: Reserved for future use
type DeviceConfig struct {
	Version       uint16 // Config format version
	Flags         uint32 // Global feature flags
	ActiveProfile uint8  // Which profile is active on boot
	Brightness    uint8  // LED brightness 0-255
	DebounceMs    uint8  // Input debounce time
	Reserved1     uint8  // Padding
	Reserved2     uint16 // Reserved for future use
}

// Errors
var (
	ErrInvalidSize = errors.New("invalid config size")
)

// Marshal writes the Profile to w in binary format.
// Returns the number of bytes written.
func (p *Profile) Marshal(w io.Writer) (int, error) {
	// Write fixed header
	header := make([]byte, 30)
	binary.LittleEndian.PutUint16(header[0:], p.Version)
	binary.LittleEndian.PutUint32(header[2:], p.Flags)
	binary.LittleEndian.PutUint32(header[6:], p.RGBColor)
	header[10] = p.RGBPattern
	header[11] = p.Reserved1
	header[12] = p.BindingCount
	header[13] = p.Reserved2
	copy(header[14:], p.Name[:])

	if _, err := w.Write(header); err != nil {
		return 0, err
	}

	// Write bindings array
	for i := range p.Bindings {
		b := make([]byte, 8)
		b[0] = uint8(p.Bindings[i].InputType)
		b[1] = p.Bindings[i].InputID
		b[2] = uint8(p.Bindings[i].OutputType)
		binary.LittleEndian.PutUint16(b[3:], p.Bindings[i].OutputValue)
		b[5] = p.Bindings[i].Modifiers
		b[6] = p.Bindings[i].Flags
		b[7] = p.Bindings[i].Reserved
		if _, err := w.Write(b); err != nil {
			return 30 + i*8, err
		}
	}

	return 286, nil
}

// Unmarshal reads the Profile from r in binary format.
func (p *Profile) Unmarshal(r io.Reader) error {
	// Read fixed header
	header := make([]byte, 30)
	if _, err := io.ReadFull(r, header); err != nil {
		return err
	}

	p.Version = binary.LittleEndian.Uint16(header[0:])
	p.Flags = binary.LittleEndian.Uint32(header[2:])
	p.RGBColor = binary.LittleEndian.Uint32(header[6:])
	p.RGBPattern = header[10]
	p.Reserved1 = header[11]
	p.BindingCount = header[12]
	p.Reserved2 = header[13]
	copy(p.Name[:], header[14:])

	// Read bindings array
	for i := range p.Bindings {
		b := make([]byte, 8)
		if _, err := io.ReadFull(r, b); err != nil {
			return err
		}
		p.Bindings[i].InputType = BindingType(b[0])
		p.Bindings[i].InputID = b[1]
		p.Bindings[i].OutputType = OutputType(b[2])
		p.Bindings[i].OutputValue = binary.LittleEndian.Uint16(b[3:])
		p.Bindings[i].Modifiers = b[5]
		p.Bindings[i].Flags = b[6]
		p.Bindings[i].Reserved = b[7]
	}

	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler for Profile.
func (p *Profile) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 286)
	binary.LittleEndian.PutUint16(buf[0:], p.Version)
	binary.LittleEndian.PutUint32(buf[2:], p.Flags)
	binary.LittleEndian.PutUint32(buf[6:], p.RGBColor)
	buf[10] = p.RGBPattern
	buf[11] = p.Reserved1
	buf[12] = p.BindingCount
	buf[13] = p.Reserved2
	copy(buf[14:30], p.Name[:])

	for i := range p.Bindings {
		offset := 30 + i*8
		buf[offset] = uint8(p.Bindings[i].InputType)
		buf[offset+1] = p.Bindings[i].InputID
		buf[offset+2] = uint8(p.Bindings[i].OutputType)
		binary.LittleEndian.PutUint16(buf[offset+3:], p.Bindings[i].OutputValue)
		buf[offset+5] = p.Bindings[i].Modifiers
		buf[offset+6] = p.Bindings[i].Flags
		buf[offset+7] = p.Bindings[i].Reserved
	}

	return buf, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler for Profile.
func (p *Profile) UnmarshalBinary(data []byte) error {
	if len(data) < 286 {
		return ErrInvalidSize
	}

	p.Version = binary.LittleEndian.Uint16(data[0:])
	p.Flags = binary.LittleEndian.Uint32(data[2:])
	p.RGBColor = binary.LittleEndian.Uint32(data[6:])
	p.RGBPattern = data[10]
	p.Reserved1 = data[11]
	p.BindingCount = data[12]
	p.Reserved2 = data[13]
	copy(p.Name[:], data[14:30])

	for i := range p.Bindings {
		offset := 30 + i*8
		p.Bindings[i].InputType = BindingType(data[offset])
		p.Bindings[i].InputID = data[offset+1]
		p.Bindings[i].OutputType = OutputType(data[offset+2])
		p.Bindings[i].OutputValue = binary.LittleEndian.Uint16(data[offset+3:])
		p.Bindings[i].Modifiers = data[offset+5]
		p.Bindings[i].Flags = data[offset+6]
		p.Bindings[i].Reserved = data[offset+7]
	}

	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler for DeviceConfig.
func (d *DeviceConfig) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint16(buf[0:], d.Version)
	binary.LittleEndian.PutUint32(buf[2:], d.Flags)
	buf[6] = d.ActiveProfile
	buf[7] = d.Brightness
	buf[8] = d.DebounceMs
	buf[9] = d.Reserved1
	binary.LittleEndian.PutUint16(buf[10:], d.Reserved2)
	return buf, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler for DeviceConfig.
func (d *DeviceConfig) UnmarshalBinary(data []byte) error {
	if len(data) < 12 {
		return ErrInvalidSize
	}

	d.Version = binary.LittleEndian.Uint16(data[0:])
	d.Flags = binary.LittleEndian.Uint32(data[2:])
	d.ActiveProfile = data[6]
	d.Brightness = data[7]
	d.DebounceMs = data[8]
	d.Reserved1 = data[9]
	d.Reserved2 = binary.LittleEndian.Uint16(data[10:])
	return nil
}

// GetName returns the profile name as a string (up to null terminator).
func (p *Profile) GetName() string {
	// Find null terminator
	for i, b := range p.Name {
		if b == 0 {
			return string(p.Name[:i])
		}
	}
	return string(p.Name[:])
}

// SetName sets the profile name from a string.
// If the name is longer than 15 bytes, it is truncated.
// The name is always null-terminated.
func (p *Profile) SetName(name string) {
	b := []byte(name)
	if len(b) > 15 {
		b = b[:15]
	}
	copy(p.Name[:], b)
	p.Name[len(b)] = 0 // Null terminate
}
