// Package protocol implements a binary serial protocol for PC app communication.
// The protocol is designed to be simple, efficient, and suitable for TinyGo.
//
// Frame format:
//
//	[SYNC:1][CMD:1][LEN:2][PAYLOAD:LEN][CRC:2]
//	- SYNC: 0xAA (frame start marker)
//	- CMD: Command byte
//	- LEN: Payload length (uint16, little-endian)
//	- PAYLOAD: Variable length data
//	- CRC: CRC16-CCITT of [CMD][LEN][PAYLOAD]
//
// Response format is identical.
package protocol

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/config"
	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/storage"
)

const (
	SyncByte = 0xAA

	// Command codes (PC → Device)
	CmdGetDeviceConfig = 0x01
	CmdSetDeviceConfig = 0x02
	CmdGetProfile      = 0x03
	CmdSetProfile      = 0x04
	CmdDeleteProfile   = 0x05
	CmdListProfiles    = 0x06
	CmdGetStorageStats = 0x07
	CmdPing            = 0x08
	CmdFactoryReset    = 0x09
	CmdGetVersion      = 0x10

	// Response status codes (Device → PC)
	StatusOK              = 0x00
	StatusError           = 0x01
	StatusInvalidCmd      = 0x02
	StatusInvalidData     = 0x03
	StatusNotFound        = 0x04
	StatusNoSpace         = 0x05
	StatusVersionMismatch = 0x06
	StatusCRCError        = 0x07
)

var (
	ErrInvalidFrame = errors.New("invalid frame")
	ErrCRCMismatch  = errors.New("CRC mismatch")
	ErrTimeout      = errors.New("timeout")
)

// Handler processes protocol commands.
type Handler struct {
	storage *storage.Manager
}

// NewHandler creates a new protocol handler.
func NewHandler(sm *storage.Manager) *Handler {
	return &Handler{
		storage: sm,
	}
}

// Frame represents a protocol frame.
type Frame struct {
	Cmd     uint8
	Payload []byte
}

// Response represents a protocol response.
type Response struct {
	Status  uint8
	Payload []byte
}

// ReadFrame reads and validates a frame from the reader.
func ReadFrame(r io.Reader) (*Frame, error) {
	// Read sync byte
	sync := make([]byte, 1)
	if _, err := io.ReadFull(r, sync); err != nil {
		return nil, err
	}
	if sync[0] != SyncByte {
		return nil, ErrInvalidFrame
	}

	// Read header (cmd + len)
	header := make([]byte, 3)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	cmd := header[0]
	length := binary.LittleEndian.Uint16(header[1:])

	// Sanity check on length
	if length > 4096 {
		return nil, ErrInvalidFrame
	}

	// Read payload
	var payload []byte
	if length > 0 {
		payload = make([]byte, length)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
	}

	// Read CRC
	crcBytes := make([]byte, 2)
	if _, err := io.ReadFull(r, crcBytes); err != nil {
		return nil, err
	}
	receivedCRC := binary.LittleEndian.Uint16(crcBytes)

	// Verify CRC
	calculatedCRC := calcCRC(append(header, payload...))
	if receivedCRC != calculatedCRC {
		return nil, ErrCRCMismatch
	}

	return &Frame{
		Cmd:     cmd,
		Payload: payload,
	}, nil
}

// WriteResponse writes a response frame to the writer.
func WriteResponse(w io.Writer, resp *Response) error {
	// Calculate total size
	payloadLen := uint16(len(resp.Payload))
	frameLen := 1 + 1 + 2 + int(payloadLen) + 2 // sync + status + len + payload + crc

	buf := make([]byte, 0, frameLen)

	// Sync byte
	buf = append(buf, SyncByte)

	// Status
	buf = append(buf, resp.Status)

	// Length
	lenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lenBytes, payloadLen)
	buf = append(buf, lenBytes...)

	// Payload
	buf = append(buf, resp.Payload...)

	// CRC (of status + len + payload)
	crc := calcCRC(buf[1:]) // Skip sync byte
	crcBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)

	_, err := w.Write(buf)
	return err
}

// WriteFrame writes a request frame (for testing/PC side).
func WriteFrame(w io.Writer, frame *Frame) error {
	payloadLen := uint16(len(frame.Payload))
	frameLen := 1 + 1 + 2 + int(payloadLen) + 2

	buf := make([]byte, 0, frameLen)

	// Sync byte
	buf = append(buf, SyncByte)

	// Command
	buf = append(buf, frame.Cmd)

	// Length
	lenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lenBytes, payloadLen)
	buf = append(buf, lenBytes...)

	// Payload
	buf = append(buf, frame.Payload...)

	// CRC
	crc := calcCRC(buf[1:])
	crcBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(crcBytes, crc)
	buf = append(buf, crcBytes...)

	_, err := w.Write(buf)
	return err
}

// Handle processes a command frame and returns a response.
func (h *Handler) Handle(frame *Frame) *Response {
	switch frame.Cmd {
	case CmdPing:
		return h.handlePing(frame.Payload)
	case CmdGetDeviceConfig:
		return h.handleGetDeviceConfig()
	case CmdSetDeviceConfig:
		return h.handleSetDeviceConfig(frame.Payload)
	case CmdGetProfile:
		return h.handleGetProfile(frame.Payload)
	case CmdSetProfile:
		return h.handleSetProfile(frame.Payload)
	case CmdDeleteProfile:
		return h.handleDeleteProfile(frame.Payload)
	case CmdListProfiles:
		return h.handleListProfiles()
	case CmdGetStorageStats:
		return h.handleGetStorageStats()
	case CmdFactoryReset:
		return h.handleFactoryReset()
	case CmdGetVersion:
		return h.handleGetVersion()
	default:
		return &Response{Status: StatusInvalidCmd}
	}
}

// handlePing responds with the same payload (echo).
func (h *Handler) handlePing(payload []byte) *Response {
	return &Response{
		Status:  StatusOK,
		Payload: payload,
	}
}

// handleGetDeviceConfig returns the current device configuration.
func (h *Handler) handleGetDeviceConfig() *Response {
	var cfg config.DeviceConfig
	if err := h.storage.LoadDevice(&cfg); err != nil {
		if err == storage.ErrProfileNotFound {
			return &Response{Status: StatusNotFound}
		}
		return &Response{Status: StatusError}
	}

	data, err := cfg.MarshalBinary()
	if err != nil {
		return &Response{Status: StatusError}
	}

	return &Response{
		Status:  StatusOK,
		Payload: data,
	}
}

// handleSetDeviceConfig updates the device configuration.
// Payload: [DeviceConfig:12 bytes]
func (h *Handler) handleSetDeviceConfig(payload []byte) *Response {
	if len(payload) != 12 {
		return &Response{Status: StatusInvalidData}
	}

	var cfg config.DeviceConfig
	if err := cfg.UnmarshalBinary(payload); err != nil {
		return &Response{Status: StatusInvalidData}
	}

	if err := h.storage.SaveDevice(&cfg); err != nil {
		if err == storage.ErrFlashFull {
			return &Response{Status: StatusNoSpace}
		}
		return &Response{Status: StatusError}
	}

	return &Response{Status: StatusOK}
}

// handleGetProfile returns a profile by slot number.
// Payload: [Slot:1 byte]
func (h *Handler) handleGetProfile(payload []byte) *Response {
	if len(payload) != 1 {
		return &Response{Status: StatusInvalidData}
	}

	slot := payload[0]

	var profile config.Profile
	if err := h.storage.LoadProfile(slot, &profile); err != nil {
		if err == storage.ErrProfileNotFound {
			return &Response{Status: StatusNotFound}
		}
		return &Response{Status: StatusError}
	}

	data, err := profile.MarshalBinary()
	if err != nil {
		return &Response{Status: StatusError}
	}

	return &Response{
		Status:  StatusOK,
		Payload: data,
	}
}

// handleSetProfile saves a profile to a slot.
// Payload: [Slot:1 byte][Profile:286 bytes]
func (h *Handler) handleSetProfile(payload []byte) *Response {
	if len(payload) != 287 {
		return &Response{Status: StatusInvalidData}
	}

	slot := payload[0]

	var profile config.Profile
	if err := profile.UnmarshalBinary(payload[1:]); err != nil {
		return &Response{Status: StatusInvalidData}
	}

	// Check version
	if profile.Version != config.CurrentVersion {
		return &Response{Status: StatusVersionMismatch}
	}

	if err := h.storage.SaveProfile(slot, &profile); err != nil {
		if err == storage.ErrFlashFull {
			return &Response{Status: StatusNoSpace}
		}
		return &Response{Status: StatusError}
	}

	return &Response{Status: StatusOK}
}

// handleDeleteProfile removes a profile from a slot.
// Payload: [Slot:1 byte]
func (h *Handler) handleDeleteProfile(payload []byte) *Response {
	if len(payload) != 1 {
		return &Response{Status: StatusInvalidData}
	}

	slot := payload[0]

	if err := h.storage.DeleteProfile(slot); err != nil {
		return &Response{Status: StatusError}
	}

	return &Response{Status: StatusOK}
}

// handleListProfiles returns all occupied profile slots.
// Response: [Count:1 byte][Slot1:1 byte][Slot2:1 byte]...
func (h *Handler) handleListProfiles() *Response {
	slots, err := h.storage.ListProfiles()
	if err != nil {
		return &Response{Status: StatusError}
	}

	payload := make([]byte, 1+len(slots))
	payload[0] = uint8(len(slots))
	for i, slot := range slots {
		payload[1+i] = slot
	}

	return &Response{
		Status:  StatusOK,
		Payload: payload,
	}
}

// handleGetStorageStats returns storage statistics.
// Response: [Total:4][Used:4][Free:4][ProfileCount:1]
func (h *Handler) handleGetStorageStats() *Response {
	stats, err := h.storage.GetStats()
	if err != nil {
		return &Response{Status: StatusError}
	}

	payload := make([]byte, 13)
	binary.LittleEndian.PutUint32(payload[0:], uint32(stats.TotalSpace))
	binary.LittleEndian.PutUint32(payload[4:], uint32(stats.UsedSpace))
	binary.LittleEndian.PutUint32(payload[8:], uint32(stats.FreeSpace))
	payload[12] = uint8(stats.ProfileCount)

	return &Response{
		Status:  StatusOK,
		Payload: payload,
	}
}

// handleFactoryReset wipes all configuration.
func (h *Handler) handleFactoryReset() *Response {
	if err := h.storage.ForceWipe(); err != nil {
		return &Response{Status: StatusError}
	}
	return &Response{Status: StatusOK}
}

// handleGetVersion returns firmware and config version info.
// Response: [FirmwareVersionMajor:1][FirmwareVersionMinor:1][ConfigVersion:2]
func (h *Handler) handleGetVersion() *Response {
	// TODO: Get firmware version from build info
	payload := make([]byte, 4)
	payload[0] = 0 // Firmware major
	payload[1] = 1 // Firmware minor
	binary.LittleEndian.PutUint16(payload[2:], config.CurrentVersion)

	return &Response{
		Status:  StatusOK,
		Payload: payload,
	}
}

// calcCRC calculates CRC16-CCITT.
// Polynomial: 0x1021, Initial: 0xFFFF
func calcCRC(data []byte) uint16 {
	var crc uint16 = 0xFFFF

	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}

	return crc
}
