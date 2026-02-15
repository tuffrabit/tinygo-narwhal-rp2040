# Tuffpad Serial Protocol

This document describes the binary serial protocol used for PC-to-device communication over USB CDC.

## Overview

The Tuffpad uses a unified binary protocol for all serial communication, including:
- Device discovery (identifying Tuffpads among connected serial devices)
- Configuration management (profiles, device settings)
- Storage operations (stats, factory reset)

This replaces the legacy text-based "areyouatuffpad?" discovery mechanism from the CircuitPython implementation.

## Protocol Format

All communication uses a consistent binary frame format:

```
[SYNC:1][CMD:1][LEN:2][PAYLOAD:LEN][CRC:2]
```

| Field | Size | Description |
|-------|------|-------------|
| SYNC | 1 byte | Frame start marker (`0xAA`) |
| CMD | 1 byte | Command code (see below) |
| LEN | 2 bytes | Payload length (uint16, little-endian) |
| PAYLOAD | LEN bytes | Command-specific data |
| CRC | 2 bytes | CRC16-CCITT of [CMD][LEN][PAYLOAD] |

### CRC16-CCITT Calculation

- Polynomial: `0x1021`
- Initial value: `0xFFFF`

## Command Codes

### PC to Device (Commands)

| Code | Command | Description |
|------|---------|-------------|
| `0x01` | GetDeviceConfig | Read current device configuration |
| `0x02` | SetDeviceConfig | Write device configuration |
| `0x03` | GetProfile | Read a profile by slot number |
| `0x04` | SetProfile | Write a profile to a slot |
| `0x05` | DeleteProfile | Remove a profile from a slot |
| `0x06` | ListProfiles | Get list of occupied profile slots |
| `0x07` | GetStorageStats | Get filesystem usage statistics |
| `0x08` | Ping | Echo test |
| `0x09` | FactoryReset | Wipe all configuration |
| `0x10` | GetVersion | Get firmware and config version info |
| `0x7F` | **Discover** | **Device identification for enumeration** |

### Device to PC (Response Status)

| Code | Status | Description |
|------|--------|-------------|
| `0x00` | OK | Success |
| `0x01` | Error | General error |
| `0x02` | InvalidCmd | Unknown command code |
| `0x03` | InvalidData | Malformed payload |
| `0x04` | NotFound | Profile/config not found |
| `0x05` | NoSpace | Insufficient storage space |
| `0x06` | VersionMismatch | Config version incompatible |
| `0x07` | CRCError | Frame CRC validation failed |

## Device Discovery

The `CmdDiscover` (`0x7F`) command is used by PC applications to identify Tuffpad devices among all connected USB serial ports.

### Discovery Request

```
AA 7F 00 00 [CRC1] [CRC2]
```

- SYNC: `0xAA`
- CMD: `0x7F` (Discover)
- LEN: `0x0000` (no payload)
- CRC: Calculated over `7F 00 00`

### Discovery Response

On success, the device responds with:

```
AA 00 07 00 74 75 66 66 70 61 64 [CRC1] [CRC2]
```

- SYNC: `0xAA`
- STATUS: `0x00` (OK)
- LEN: `0x0007` (7 bytes)
- PAYLOAD: `"tuffpad"` (ASCII)
- CRC: Calculated over `00 07 00 74 75 66 66 70 61 64`

### PC Discovery Algorithm

```python
import serial
import struct

def crc16_ccitt(data: bytes) -> int:
    crc = 0xFFFF
    for byte in data:
        crc ^= byte << 8
        for _ in range(8):
            if crc & 0x8000:
                crc = (crc << 1) ^ 0x1021
            else:
                crc <<= 1
        crc &= 0xFFFF
    return crc

def build_discover_frame() -> bytes:
    header = bytes([0x7F, 0x00, 0x00])  # CMD + LEN (2 bytes, LE)
    crc = crc16_ccitt(header)
    return bytes([0xAA]) + header + struct.pack('<H', crc)

def parse_response(frame: bytes) -> tuple[int, bytes]:
    if len(frame) < 6:
        raise ValueError("Frame too short")
    if frame[0] != 0xAA:
        raise ValueError("Invalid sync byte")
    
    status = frame[1]
    length = struct.unpack('<H', frame[2:4])[0]
    payload = frame[4:4+length]
    received_crc = struct.unpack('<H', frame[4+length:6+length])[0]
    
    # Verify CRC (over status + len + payload)
    crc_data = frame[1:4+length]
    if crc16_ccitt(crc_data) != received_crc:
        raise ValueError("CRC mismatch")
    
    return status, payload

def is_tuffpad(port_path: str) -> bool:
    try:
        with serial.Serial(port_path, baudrate=115200, timeout=0.5) as port:
            # Send discovery frame
            port.write(build_discover_frame())
            
            # Read response (minimum 6 bytes, up to max frame size)
            response = port.read(128)
            
            status, payload = parse_response(response)
            
            if status == 0 and payload == b'tuffpad':
                return True
    except:
        pass
    return False

# Enumerate all Tuffpads
def find_tuffpads():
    import serial.tools.list_ports
    
    tuffpads = []
    for port in serial.tools.list_ports.comports():
        if is_tuffpad(port.device):
            tuffpads.append(port.device)
    return tuffpads
```

## Configuration Commands

### GetDeviceConfig (0x01)

Request current device configuration.

**Request:** `AA 01 00 00 [CRC]`

**Response (12 bytes):**
- `Version` (2 bytes): Config format version
- `Flags` (4 bytes): Device feature flags
- `ActiveProfile` (1 byte): Currently selected profile slot
- `Brightness` (1 byte): LED brightness (0-255)
- `DebounceMs` (2 bytes): Input debounce time in milliseconds
- Reserved (2 bytes)

### SetDeviceConfig (0x02)

Write device configuration.

**Request:** `AA 02 0C 00 [12 bytes config] [CRC]`

**Response:** `AA 00 00 00 [CRC]` (OK) or error status

### GetProfile (0x03)

Read a profile by slot number.

**Request:** `AA 03 01 00 [slot] [CRC]`

**Response (286 bytes):** Profile data structure

### SetProfile (0x04)

Write a profile to a slot.

**Request:** `AA 04 1F 01 [slot] [286 bytes profile] [CRC]`
(287 bytes total payload: 1 byte slot + 286 bytes profile)

**Response:** `AA 00 00 00 [CRC]` (OK) or error status

## Architecture Notes

### Goroutine Model

The serial protocol handler runs in its own goroutine:

```go
// main.go
go mainSerial.Handle()
```

This goroutine:
1. Blocks on `protocol.ReadFrame()` waiting for USB data
2. The TinyGo scheduler yields this goroutine when blocked
3. When data arrives, the frame is parsed and dispatched
4. Response is sent via `protocol.WriteResponse()`
5. Loop continues immediately to handle next frame

### Integration with Storage

The protocol handler is initialized with a storage manager:

```go
storageMgr, _ := storage.New(machine.Flash, true)
protoHandler := protocol.NewHandler(storageMgr)
mainSerial := serial.NewSerial(machine.Serial, protoHandler)
```

All configuration commands operate on the LittleFS filesystem in the RP2040's on-chip flash.

### Error Handling

- **Invalid frames** (wrong sync byte, CRC mismatch): Silently discarded, loop continues
- **Invalid commands**: Return `StatusInvalidCmd`
- **Storage errors**: Return appropriate error status
- **Read timeouts**: Handled by `ReadFrame` returning error, loop continues

## Migration from Legacy Protocol

If you have PC code using the old text-based discovery:

**Before:**
```python
port.write(b"areyouatuffpad?\n")
response = port.readline()
if response.strip() == b"areyouatuffpad?yes":
    # It's a Tuffpad
```

**After:**
```python
port.write(build_discover_frame())
status, payload = parse_response(port.read(32))
if status == 0 and payload == b'tuffpad':
    # It's a Tuffpad
```

## References

- `pkg/protocol/protocol.go` - Protocol implementation
- `serial/serial.go` - Serial I/O handler
- `goroutine architecture.md` - Scheduling and task design
