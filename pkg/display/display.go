//go:build !nodebug

// Package display provides SSD1306 OLED display support for debug output.
// It shows serial communication activity with incoming frames on the yellow
// rows (0-1) and outgoing responses on the blue rows (2-3).
//
// To build without display support (saves ~1KB RAM and flash), use:
//   tinygo build -tags=nodebug -target=pico -o firmware.uf2 .
package display

import (
	"fmt"
	"image/color"
	"machine"
	"time"

	"tinygo.org/x/drivers/ssd1306"
)

const (
	// I2C configuration
	i2cAddress = 0x3C
	sclPin     = machine.GPIO1
	sdaPin     = machine.GPIO0

	// Display dimensions
	screenWidth  = 128
	screenHeight = 64
	charWidth    = 8
	charHeight   = 8
	cols         = screenWidth / charWidth  // 16 columns
	rows         = screenHeight / charHeight // 8 rows

	// Row assignments
	rowInBytes   = 0 // Yellow - incoming raw bytes
	rowInParsed  = 1 // Yellow - incoming parsed
	rowOutBytes  = 2 // Blue - outgoing raw bytes
	rowOutParsed = 3 // Blue - outgoing parsed
)

// Colors for monochrome display
var (
	black = color.RGBA{0, 0, 0, 0}
	white = color.RGBA{255, 255, 255, 255}
)

// Manager handles the SSD1306 display for debug output.
type Manager struct {
	device *ssd1306.Device
	i2c    *machine.I2C
	buffer [rows][cols]byte
}

// NewManager creates and initializes the display manager.
// Returns nil if display initialization fails (non-fatal for debug).
func NewManager() *Manager {
	// Initialize I2C bus
	i2c := machine.I2C0
	if err := i2c.Configure(machine.I2CConfig{
		Frequency: 400000, // 400kHz fast mode
		SCL:       sclPin,
		SDA:       sdaPin,
	}); err != nil {
		fmt.Printf("I2C config failed: %v\n", err)
		return nil
	}

	// Small delay for bus stabilization
	time.Sleep(10 * time.Millisecond)

	// Initialize SSD1306
	dev := ssd1306.NewI2C(i2c)
	dev.Configure(ssd1306.Config{
		Address: i2cAddress,
		Width:   screenWidth,
		Height:  screenHeight,
	})

	// Clear display
	dev.ClearDisplay()

	mgr := &Manager{
		device: dev,
		i2c:    i2c,
	}

	// Show initial message
	mgr.drawString(0, 0, "TuffPad Debug")
	mgr.drawString(0, 1, "Waiting for data...")
	mgr.refresh()

	return mgr
}

// ShowIncomingFrame displays an incoming serial frame on the yellow rows.
// bytesRow shows the raw hex bytes, parsedRow shows human-readable info.
func (m *Manager) ShowIncomingFrame(bytesStr, parsedStr string) {
	m.clearRow(rowInBytes)
	m.clearRow(rowInParsed)
	m.drawString(0, rowInBytes, truncate("I:"+bytesStr, cols-1))
	m.drawString(0, rowInParsed, truncate(" "+parsedStr, cols-1))
	m.refresh()
}

// ShowOutgoingResponse displays an outgoing serial response on the blue rows.
// bytesRow shows the raw hex bytes, parsedRow shows human-readable info.
func (m *Manager) ShowOutgoingResponse(bytesStr, parsedStr string) {
	m.clearRow(rowOutBytes)
	m.clearRow(rowOutParsed)
	m.drawString(0, rowOutBytes, truncate("O:"+bytesStr, cols-1))
	m.drawString(0, rowOutParsed, truncate(" "+parsedStr, cols-1))
	m.refresh()
}

// ShowError displays an error message on the display.
func (m *Manager) ShowError(msg string) {
	m.clearRow(rowOutBytes)
	m.clearRow(rowOutParsed)
	m.drawString(0, rowOutBytes, "ERR:")
	m.drawString(0, rowOutParsed, truncate(msg, cols-1))
	m.refresh()
}

// clearRow clears a single row in the buffer and on the display.
func (m *Manager) clearRow(row int) {
	if row < 0 || row >= rows {
		return
	}
	for col := 0; col < cols; col++ {
		m.buffer[row][col] = 0
	}
	// Clear the actual display row (all 8 pixel lines)
	yStart := int16(row * charHeight)
	for y := yStart; y < yStart+charHeight; y++ {
		for x := int16(0); x < screenWidth; x++ {
			m.device.SetPixel(x, y, black)
		}
	}
}

// drawString draws a string at the specified column and row.
func (m *Manager) drawString(col, row int, s string) {
	if row < 0 || row >= rows {
		return
	}
	for i, ch := range s {
		c := col + i
		if c < 0 || c >= cols {
			continue
		}
		m.buffer[row][c] = byte(ch)
		m.drawChar(c, row, byte(ch))
	}
}

// drawChar draws a single character at the specified column and row.
func (m *Manager) drawChar(col, row int, ch byte) {
	if col < 0 || col >= cols || row < 0 || row >= rows {
		return
	}

	// Get font bitmap for this character
	bitmap := font8x8[ch]

	// Draw to display
	// x, y is the top-left corner of the character
	xStart := int16(col * charWidth)
	yStart := int16(row * charHeight)

	// Font data: 8 bytes per character, each byte is a horizontal row (8 pixels wide)
	// byte 0 = top row, byte 7 = bottom row
	// bit 0 = leftmost pixel, bit 7 = rightmost pixel
	for rowOffset := 0; rowOffset < 8; rowOffset++ {
		byteVal := bitmap[rowOffset]
		y := yStart + int16(rowOffset)
		for bit := 0; bit < 8; bit++ {
			on := (byteVal >> uint(bit)) & 1
			x := xStart + int16(bit)
			if on != 0 {
				m.device.SetPixel(x, y, white)
			} else {
				m.device.SetPixel(x, y, black)
			}
		}
	}
}

// refresh updates the display with current buffer content.
func (m *Manager) refresh() {
	m.device.Display()
}

// truncate limits a string to maxLen characters, adding ".." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 2 {
		return s[:maxLen]
	}
	return s[:maxLen-2] + ".."
}
