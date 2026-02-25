//go:build nodebug

// Package display provides a no-op stub when built with the nodebug tag.
// This saves memory by excluding the SSD1306 driver and display code.
//
// To build without display support, use:
//   tinygo build -tags=nodebug -target=pico -o firmware.uf2 .
package display

// Manager is a no-op stub when nodebug build tag is used.
type Manager struct{}

// NewManager returns nil when nodebug build tag is used.
// The serial handler will handle nil display gracefully.
func NewManager() *Manager {
	return nil
}

// ShowIncomingFrame is a no-op in nodebug mode.
func (m *Manager) ShowIncomingFrame(bytesStr, parsedStr string) {}

// ShowOutgoingResponse is a no-op in nodebug mode.
func (m *Manager) ShowOutgoingResponse(bytesStr, parsedStr string) {}

// ShowError is a no-op in nodebug mode.
func (m *Manager) ShowError(msg string) {}
