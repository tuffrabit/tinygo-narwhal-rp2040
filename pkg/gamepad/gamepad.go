// Package gamepad implements a USB HID Gamepad device using Report ID 4
// This is designed to work with the composite HID descriptor
package gamepad

import (
	"machine"
	"machine/usb/hid"
)

// Button represents a gamepad button (0-15)
type Button uint8

// Standard gamepad button mapping
const (
	ButtonA      Button = 0
	ButtonB      Button = 1
	ButtonX      Button = 2
	ButtonY      Button = 3
	ButtonL1     Button = 4
	ButtonR1     Button = 5
	ButtonL2     Button = 6
	ButtonR2     Button = 7
	ButtonSelect Button = 8
	ButtonStart  Button = 9
	ButtonL3     Button = 10 // Left stick click
	ButtonR3     Button = 11 // Right stick click
	ButtonUp     Button = 12
	ButtonDown   Button = 13
	ButtonLeft   Button = 14
	ButtonRight  Button = 15
)

// Axis represents an analog axis (0-3)
type Axis uint8

const (
	AxisX  Axis = 0 // Left stick X
	AxisY  Axis = 1 // Left stick Y
	AxisZ  Axis = 2 // Right stick X
	AxisRz Axis = 3 // Right stick Y
)

// Gamepad represents a USB HID Gamepad device
type Gamepad struct {
	buttons uint16 // 16 buttons as bits
	axes    [4]int8 // X, Y, Z, Rz
	buf     *hid.RingBuffer
	waitTxc bool
}

// gamepad is the singleton instance
var gamepadInstance *Gamepad

// init registers the gamepad with the HID subsystem
func init() {
	if gamepadInstance == nil {
		gamepadInstance = &Gamepad{
			buf: hid.NewRingBuffer(),
		}
		// Register with HID - this works with the standard TinyGo hid package
		// because we're using Report ID 4 which the host will route correctly
		hid.SetHandler(gamepadInstance)
	}
}

// Port returns the gamepad instance
func Port() *Gamepad {
	return gamepadInstance
}

// New creates a new gamepad instance (alternative to Port())
func New() *Gamepad {
	return Port()
}

// TxHandler is called by the USB interrupt when the endpoint is ready to transmit
// This implements the hidDevicer interface
func (g *Gamepad) TxHandler() bool {
	g.waitTxc = false
	if b, ok := g.buf.Get(); ok {
		g.waitTxc = true
		hid.SendUSBPacket(b)
		return true
	}
	return false
}

// RxHandler handles output reports from the host (if any)
// This implements the hidDevicer interface
func (g *Gamepad) RxHandler(b []byte) bool {
	// Gamepad typically doesn't receive output reports
	// But we could handle force feedback here if implemented
	return false
}

// tx sends a report packet, queuing if necessary
func (g *Gamepad) tx(b []byte) {
	if machine.USBDev.InitEndpointComplete {
		if g.waitTxc {
			// USB busy, queue for later
			g.buf.Put(b)
		} else {
			// Send immediately
			g.waitTxc = true
			hid.SendUSBPacket(b)
		}
	}
}

// SetButton sets the state of a button
func (g *Gamepad) SetButton(button Button, pressed bool) {
	if button > 15 {
		return
	}
	if pressed {
		g.buttons |= (1 << button)
	} else {
		g.buttons &^= (1 << button)
	}
}

// Press presses a button (convenience method)
func (g *Gamepad) Press(button Button) {
	g.SetButton(button, true)
}

// Release releases a button (convenience method)
func (g *Gamepad) Release(button Button) {
	g.SetButton(button, false)
}

// IsPressed returns true if a button is currently pressed
func (g *Gamepad) IsPressed(button Button) bool {
	if button > 15 {
		return false
	}
	return (g.buttons & (1 << button)) != 0
}

// SetAxis sets an axis value (-127 to 127)
func (g *Gamepad) SetAxis(axis Axis, value int8) {
	if axis > 3 {
		return
	}
	g.axes[axis] = value
}

// SetAxisInt sets an axis value from an integer (clamped to -127..127)
func (g *Gamepad) SetAxisInt(axis Axis, value int) {
	if value < -127 {
		value = -127
	} else if value > 127 {
		value = 127
	}
	g.SetAxis(axis, int8(value))
}

// GetAxis returns the current axis value
func (g *Gamepad) GetAxis(axis Axis) int8 {
	if axis > 3 {
		return 0
	}
	return g.axes[axis]
}

// Reset clears all button and axis states
func (g *Gamepad) Reset() {
	g.buttons = 0
	g.axes = [4]int8{0, 0, 0, 0}
}

// SendState sends the current gamepad state to the host
// This should be called after updating buttons/axes
func (g *Gamepad) SendState() {
	// Report format (7 bytes):
	// Byte 0: Report ID (4)
	// Byte 1: Buttons low byte (buttons 0-7)
	// Byte 2: Buttons high byte (buttons 8-15)
	// Byte 3: X axis
	// Byte 4: Y axis
	// Byte 5: Z axis
	// Byte 6: Rz axis
	g.tx([]byte{
		0x04,                 // Report ID 4
		byte(g.buttons),      // Buttons low
		byte(g.buttons >> 8), // Buttons high
		byte(g.axes[0]),      // X
		byte(g.axes[1]),      // Y
		byte(g.axes[2]),      // Z
		byte(g.axes[3]),      // Rz
	})
}

// Send sends the gamepad state (alias for SendState)
func (g *Gamepad) Send() {
	g.SendState()
}

// Note: The gamepad state is only sent when SendState() is called.
// This allows atomic updates - set multiple buttons/axes, then send once.

// Example usage:
//
//   gp := gamepad.Port()
//   gp.SetButton(gamepad.ButtonA, true)
//   gp.SetAxis(gamepad.AxisX, 100)
//   gp.SendState()
