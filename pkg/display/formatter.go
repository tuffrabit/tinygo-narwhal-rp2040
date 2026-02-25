package display

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/protocol"
)

// FrameFormatter formats protocol frames for display on the SSD1306.
// It creates compact string representations suitable for 16-character wide display rows.
type FrameFormatter struct{}

// NewFrameFormatter creates a new frame formatter.
func NewFrameFormatter() *FrameFormatter {
	return &FrameFormatter{}
}

// FormatIncoming formats an incoming request frame for display.
// Returns bytes string and parsed string.
func (f *FrameFormatter) FormatIncoming(frame *protocol.Frame) (bytesStr, parsedStr string) {
	// Format raw bytes (sync byte not included in frame, but we show it conceptually)
	bytesStr = f.formatFrameBytes(frame.Cmd, frame.Payload)

	// Format parsed info
	cmdName := f.getCommandName(frame.Cmd)
	payloadLen := len(frame.Payload)
	parsedStr = fmt.Sprintf("%s[%d]", cmdName, payloadLen)

	return bytesStr, parsedStr
}

// FormatOutgoing formats an outgoing response frame for display.
// Returns bytes string and parsed string.
func (f *FrameFormatter) FormatOutgoing(resp *protocol.Response) (bytesStr, parsedStr string) {
	// Format raw bytes
	bytesStr = f.formatResponseBytes(resp.Status, resp.Payload)

	// Format parsed info
	statusName := f.getStatusName(resp.Status)
	payloadLen := len(resp.Payload)
	parsedStr = fmt.Sprintf("%s[%d]", statusName, payloadLen)

	return bytesStr, parsedStr
}

// FormatError formats an error for display.
func (f *FrameFormatter) FormatError(err error) string {
	msg := err.Error()
	if len(msg) > 12 {
		msg = msg[:12]
	}
	return msg
}

// formatFrameBytes formats the raw bytes of a frame as hex.
// Format: AA CMD LEN_LO LEN_HI [PAYLOAD] CRC_LO CRC_HI
func (f *FrameFormatter) formatFrameBytes(cmd uint8, payload []byte) string {
	var b strings.Builder

	// Calculate total frame size
	payloadLen := uint16(len(payload))

	// Sync byte
	b.WriteString(fmt.Sprintf("%02X ", protocol.SyncByte))

	// Command
	b.WriteString(fmt.Sprintf("%02X ", cmd))

	// Length (2 bytes, little-endian)
	lenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lenBytes, payloadLen)
	b.WriteString(fmt.Sprintf("%02X%02X ", lenBytes[0], lenBytes[1]))

	// Payload (first few bytes only)
	maxPayloadBytes := 4 // Show max 4 payload bytes to fit on display
	for i := 0; i < len(payload) && i < maxPayloadBytes; i++ {
		b.WriteString(fmt.Sprintf("%02X", payload[i]))
	}
	if len(payload) > maxPayloadBytes {
		b.WriteString("..")
	} else if len(payload) > 0 {
		b.WriteString(" ")
	}

	// CRC placeholder (we don't calculate it here, just show dots)
	b.WriteString("..")

	return b.String()
}

// formatResponseBytes formats the raw bytes of a response as hex.
// Format: AA STATUS LEN_LO LEN_HI [PAYLOAD] CRC_LO CRC_HI
func (f *FrameFormatter) formatResponseBytes(status uint8, payload []byte) string {
	var b strings.Builder

	// Calculate total frame size
	payloadLen := uint16(len(payload))

	// Sync byte
	b.WriteString(fmt.Sprintf("%02X ", protocol.SyncByte))

	// Status
	b.WriteString(fmt.Sprintf("%02X ", status))

	// Length (2 bytes, little-endian)
	lenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lenBytes, payloadLen)
	b.WriteString(fmt.Sprintf("%02X%02X ", lenBytes[0], lenBytes[1]))

	// Payload (first few bytes only)
	maxPayloadBytes := 4 // Show max 4 payload bytes to fit on display
	for i := 0; i < len(payload) && i < maxPayloadBytes; i++ {
		b.WriteString(fmt.Sprintf("%02X", payload[i]))
	}
	if len(payload) > maxPayloadBytes {
		b.WriteString("..")
	} else if len(payload) > 0 {
		b.WriteString(" ")
	}

	// CRC placeholder
	b.WriteString("..")

	return b.String()
}

// getCommandName returns a short name for a command code.
func (f *FrameFormatter) getCommandName(cmd uint8) string {
	switch cmd {
	case protocol.CmdGetDeviceConfig:
		return "GetDevCfg"
	case protocol.CmdSetDeviceConfig:
		return "SetDevCfg"
	case protocol.CmdGetProfile:
		return "GetProf"
	case protocol.CmdSetProfile:
		return "SetProf"
	case protocol.CmdDeleteProfile:
		return "DelProf"
	case protocol.CmdListProfiles:
		return "LstProf"
	case protocol.CmdGetStorageStats:
		return "GetStor"
	case protocol.CmdPing:
		return "Ping"
	case protocol.CmdFactoryReset:
		return "FctRst"
	case protocol.CmdGetVersion:
		return "GetVer"
	case protocol.CmdDiscover:
		return "Discvr"
	default:
		return fmt.Sprintf("Cmd%02X", cmd)
	}
}

// getStatusName returns a short name for a status code.
func (f *FrameFormatter) getStatusName(status uint8) string {
	switch status {
	case protocol.StatusOK:
		return "OK"
	case protocol.StatusError:
		return "Err"
	case protocol.StatusInvalidCmd:
		return "InvCmd"
	case protocol.StatusInvalidData:
		return "InvData"
	case protocol.StatusNotFound:
		return "NotFnd"
	case protocol.StatusNoSpace:
		return "NoSpace"
	case protocol.StatusVersionMismatch:
		return "VerMis"
	case protocol.StatusCRCError:
		return "CRC"
	default:
		return fmt.Sprintf("Sts%02X", status)
	}
}
