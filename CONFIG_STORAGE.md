# Gamepad Configuration Storage System

This document describes the persistent configuration storage system for the TuffPad gamepad, designed for the RP2040 microcontroller with embedded flash storage.

## Table of Contents

- [Overview](#overview)
- [Design Decisions](#design-decisions)
- [Hardware Constraints](#hardware-constraints)
- [Implementation Details](#implementation-details)
  - [Data Structures](#data-structures)
  - [Storage Layout](#storage-layout)
  - [Atomic Writes](#atomic-writes)
  - [Version Management](#version-management)
- [Serial Protocol](#serial-protocol)
- [Usage Examples](#usage-examples)
- [PC App Integration](#pc-app-integration)
- [Migration Strategy](#migration-strategy)

---

## Overview

The configuration storage system provides persistent storage for device settings and keybinding profiles using LittleFS on the RP2040's embedded flash. It supports:

- **Dynamic profile count** - No compile-time limit, constrained only by available flash
- **Wear leveling** - LittleFS distributes writes across flash blocks
- **Power-loss safety** - Atomic writes ensure config integrity
- **Version management** - Automatic config wipe on firmware update (intentional)
- **Zero-allocation serialization** - Fixed-size binary format for minimal RAM usage

---

## Design Decisions

### Questions Answered

**1. How often will users edit configurations?**

End user usage patterns vary, but we must assume worst-case: frequent, continuous editing throughout the device lifespan. LittleFS provides wear leveling to handle this.

**2. How many profiles and bindings?**

- **Minimum profiles:** 10 (users use profiles as temporary binding layers)
- **No hard limit:** Query filesystem at runtime for available space
- **Bindings per profile:** 32 (21 keys + joystick + D-pad + RGB settings + mode flags)

**3. How to handle config format updates?**

Users back up configs via PC app before flashing new firmware. The device automatically wipes configs on version mismatch. Users restore configs after firmware update via the PC app.

**4. How will users edit configs?**

Via USB CDC serial using a PC-side configuration application.

**5. How safe should the storage be?**

Reasonably safe to avoid user frustration. Atomic writes prevent corruption, but users have backup/restore capability through the PC app for major changes.

**6. Additional requirement: Config versioning**

The flash stores a config version. On boot, if the firmware's expected version differs from flash, configs are wiped. The PC app handles version conversion during backup/restore.

---

## Hardware Constraints

| Parameter | Value |
|-----------|-------|
| **Total Flash** | 2MB (W25Q16JV or similar) |
| **Erase Block Size** | 4096 bytes |
| **Write Block Size** | 256 bytes |
| **Available Storage** | From end of program code to end of flash |
| **RAM** | 256KB |

LittleFS overhead: ~4-8KB RAM for cache/lookahead.

---

## Implementation Details

### Data Structures

All structs use fixed-size arrays for zero-allocation binary serialization.

#### DeviceConfig (12 bytes)

```go
type DeviceConfig struct {
    Version       uint16  // Config format version
    Flags         uint32  // Global feature flags
    ActiveProfile uint8   // Which profile is active on boot
    Brightness    uint8   // LED brightness 0-255
    DebounceMs    uint8   // Input debounce time
    Reserved1     uint8   // Padding
    Reserved2     uint16  // Reserved for future use
}
```

#### Profile (286 bytes)

```go
type Profile struct {
    Version      uint16         // Config format version
    Flags        uint32         // Profile-level flags
    RGBColor     uint32         // RGB LED color (RGB888)
    RGBPattern   uint8          // RGB pattern ID
    Reserved1    uint8          // Padding
    BindingCount uint8          // Active bindings (<= 32)
    Reserved2    uint8          // Padding
    Name         [16]byte       // UTF-8 name (null-terminated)
    Bindings     [32]KeyBinding // Fixed array of bindings
}
```

#### KeyBinding (8 bytes)

```go
type KeyBinding struct {
    InputType   BindingType  // Key, JoystickButton, DPad, RGBPattern
    InputID     uint8        // Which input (0-31)
    OutputType  OutputType   // Keyboard, GamepadButton, MouseButton, Consumer
    OutputValue uint16       // HID keycode or button mask
    Modifiers   uint8        // Ctrl/Shift/Alt/Gui
    Flags       uint8        // Tap/Hold/Double-tap
    Reserved    uint8        // Padding
}
```

### Storage Layout

```
┌─────────────────────────────────────────────────────────────┐
│                    FLASH LAYOUT (2MB)                        │
├─────────────────────────────────────────────────────────────┤
│  [Program Code + Static Data]  (~256KB typical)              │
├─────────────────────────────────────────────────────────────┤
│  [LittleFS Partition]                                         │
│  ├── /config/                                                 │
│  │   ├── device.bin          (12 bytes + metadata)           │
│  │   └── profiles/                                           │
│  │       ├── 0.bin                                           │
│  │       ├── 3.bin                                           │
│  │       ├── 7.bin                                           │
│  │       └── 12.bin                                          │
├─────────────────────────────────────────────────────────────┤
│  [Unused / Available for profiles]                           │
└─────────────────────────────────────────────────────────────┘
```

### Atomic Writes

All configuration writes use atomic file operations:

1. Write to temporary file (`profile.bin.tmp`)
2. Sync to ensure data hits flash
3. Remove old file (if exists)
4. Rename temp file to final name

This ensures the original file is never in a partially written state. If power is lost during write, the temp file is cleaned up on next boot.

### Version Management

```go
const CurrentVersion uint16 = 1
```

**Boot sequence:**
1. Mount filesystem
2. Check stored config version
3. If version != `CurrentVersion`, wipe all configs
4. Clean up any orphaned temp files

**Why wipe instead of migrate?**
- Simplifies firmware code
- Forces conscious user action through PC app
- Avoids complex on-device migration logic

---

## Serial Protocol

Binary protocol over USB CDC with CRC protection.

### Frame Format

```
[SYNC:1][CMD:1][LEN:2][PAYLOAD:LEN][CRC:2]
```

- **SYNC**: `0xAA` (frame start marker)
- **CMD**: Command byte
- **LEN**: Payload length (uint16, little-endian)
- **CRC**: CRC16-CCITT of `[CMD][LEN][PAYLOAD]`

### Commands

| Code | Command | Request | Response |
|------|---------|---------|----------|
| `0x01` | GET_DEVICE_CONFIG | - | DeviceConfig (12 bytes) |
| `0x02` | SET_DEVICE_CONFIG | DeviceConfig (12 bytes) | Status |
| `0x03` | GET_PROFILE | Slot (1 byte) | Profile (286 bytes) |
| `0x04` | SET_PROFILE | Slot + Profile (287 bytes) | Status |
| `0x05` | DELETE_PROFILE | Slot (1 byte) | Status |
| `0x06` | LIST_PROFILES | - | Count + Slots |
| `0x07` | GET_STORAGE_STATS | - | Total + Used + Free + Count |
| `0x08` | PING | Any | Echo |
| `0x09` | FACTORY_RESET | - | Status |
| `0x10` | GET_VERSION | - | FW Major + FW Minor + Config Version |

### Status Codes

| Code | Meaning |
|------|---------|
| `0x00` | OK |
| `0x01` | Generic Error |
| `0x02` | Invalid Command |
| `0x03` | Invalid Data |
| `0x04` | Not Found |
| `0x05` | No Space |
| `0x06` | Version Mismatch |
| `0x07` | CRC Error |

---

## Usage Examples

### Initialize Storage

```go
import (
    "machine"
    "github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/storage"
)

func main() {
    // Use RP2040's on-board flash
    blockDev := machine.Flash
    
    // Initialize (format if needed)
    mgr, err := storage.New(blockDev, true)
    if err != nil {
        // Handle error - flash may be corrupted
    }
    defer mgr.Close()
    
    // Storage is ready
}
```

### Save Device Configuration

```go
import "github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/config"

deviceCfg := config.DeviceConfig{
    Flags:         0x00000001,  // Enable some feature
    ActiveProfile: 3,
    Brightness:    200,
    DebounceMs:    5,
}

if err := mgr.SaveDevice(&deviceCfg); err != nil {
    // Handle error
}
```

### Create a Profile

```go
profile := config.Profile{
    Flags:        0x00000001,  // KB mode enabled
    RGBColor:     0xFF00FF,    // Magenta
    RGBPattern:   2,           // Breathing pattern
    BindingCount: 2,
}
profile.SetName("Gaming")

// Bind physical key 0 to HID 'A'
profile.Bindings[0] = config.KeyBinding{
    InputType:   config.BindingTypeKey,
    InputID:     0,
    OutputType:  config.OutputTypeKeyboard,
    OutputValue: 0x04,  // 'a' key
    Modifiers:   0,
    Flags:       0,
}

// Bind joystick button to gamepad Button A
profile.Bindings[1] = config.KeyBinding{
    InputType:   config.BindingTypeJoystickButton,
    InputID:     0,
    OutputType:  config.OutputTypeGamepadButton,
    OutputValue: 1 << 0,  // Button A
    Modifiers:   0,
    Flags:       0,
}

// Save to slot 5
if err := mgr.SaveProfile(5, &profile); err != nil {
    // Handle error (e.g., no space)
}
```

### Load Active Profile on Boot

```go
var deviceCfg config.DeviceConfig
if err := mgr.LoadDevice(&deviceCfg); err != nil {
    // Use defaults - first boot or wiped
    deviceCfg.ActiveProfile = 0
    deviceCfg.Brightness = 255
}

var activeProfile config.Profile
if err := mgr.LoadProfile(deviceCfg.ActiveProfile, &activeProfile); err != nil {
    // Profile not found, create default
    activeProfile = createDefaultProfile()
}

// Cache in RAM for runtime use
inputHandler.SetProfile(activeProfile)
```

### List All Profiles

```go
slots, err := mgr.ListProfiles()
if err != nil {
    // Handle error
}

for _, slot := range slots {
    var profile config.Profile
    if err := mgr.LoadProfile(slot, &profile); err == nil {
        name := profile.GetName()
        // Send to PC app or display on device
    }
}
```

---

## PC App Integration

### Backup Flow

1. Connect to device over USB CDC
2. Send `GET_VERSION` to check config version
3. Send `GET_DEVICE_CONFIG` to retrieve global settings
4. Send `LIST_PROFILES` to get occupied slots
5. For each slot, send `GET_PROFILE` to retrieve profile data
6. Store all data to disk on PC

### Restore Flow

1. Connect to device over USB CDC
2. Send `GET_VERSION` to check firmware's config version
3. If PC backup version != firmware version:
   - Convert config format (app-specific logic)
4. Send `SET_DEVICE_CONFIG` with global settings
5. For each profile in backup:
   - Send `SET_PROFILE` to restore
6. If errors occur (e.g., no space), notify user

### Firmware Update Flow

1. User backs up configs via PC app
2. User puts device in bootloader mode
3. User flashes new firmware
4. On first boot, device detects version mismatch and wipes configs
5. User restores configs via PC app (with conversion if needed)

---

## Migration Strategy

### When Config Version Changes

1. Bump `CurrentVersion` constant in firmware
2. Update PC app to:
   - Recognize new version
   - Convert old format to new format on restore
3. Document changes in release notes

### Example Conversion (PC App)

```go
func ConvertV1ToV2(v1 ProfileV1) ProfileV2 {
    return ProfileV2{
        Version: 2,
        // Copy fields
        RGBColor: v1.RGBColor,
        // Map old fields to new
        NewField: defaultValue,
    }
}
```

---

## Memory and Performance

### RAM Usage

| Component | Size |
|-----------|------|
| LittleFS cache | 512 bytes |
| LittleFS lookahead | 128 bytes |
| Profile (loaded) | 286 bytes |
| DeviceConfig | 12 bytes |
| **Total typical** | **~1KB** |

### Flash Usage

| Item | Size |
|------|------|
| Device config | ~44 bytes (with overhead) |
| Per profile | ~320 bytes (with overhead) |
| 10 profiles | ~3.2KB |
| 50 profiles | ~16KB |

### Performance

| Operation | Typical Time |
|-----------|--------------|
| Load profile | < 1ms |
| Save profile | 5-20ms (includes flash erase/write) |
| List profiles | < 5ms |
| Mount filesystem | 10-50ms |

---

## Testing

Run tests with standard Go (uses memory-backed block device):

```bash
go test ./pkg/config/...
go test ./pkg/storage/...
go test ./pkg/protocol/...
```

All tests use `tinyfs.NewMemoryDevice(256, 4096, 64)` to simulate RP2040 flash without requiring actual hardware.

---

## Future Enhancements

Potential improvements for future versions:

1. **Compression**: Compress profiles to save flash space
2. **Encryption**: Encrypt sensitive config data
3. **Differential updates**: Only write changed fields
4. **Wear tracking**: Monitor flash wear and report to PC app
5. **Auto-backup**: Trigger PC backup on config change
