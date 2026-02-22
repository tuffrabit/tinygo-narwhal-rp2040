package serial

import (
	"io"
	"machine"
	"time"

	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/protocol"
)

// Serial handles USB CDC communication using the binary protocol.
type Serial struct {
	serial  machine.Serialer
	handler *protocol.Handler
}

// NewSerial creates a new Serial handler.
func NewSerial(serial machine.Serialer, handler *protocol.Handler) Serial {
	return Serial{
		serial:  serial,
		handler: handler,
	}
}

// dtrWaiter is the interface for checking DTR status.
// machine.USBCDC implements this.
type dtrWaiter interface {
	DTR() bool
}

// waitForDTR blocks until DTR is asserted or timeout.
// This ensures the host serial port is fully open before we send responses.
func waitForDTR(serial machine.Serialer, timeout time.Duration) bool {
	dtrChecker, ok := serial.(dtrWaiter)
	if !ok {
		// If we can't check DTR, just proceed
		return true
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if dtrChecker.DTR() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// Handle runs the serial read/write loop.
// This should be called in its own goroutine.
func (s *Serial) Handle() {
	reader := &serialReader{serial: s.serial}

	// Wait for DTR to be asserted before processing commands.
	// TinyGo's USB CDC drops writes if DTR is not set, which causes
	// PC apps to receive no response (frame too short: 0 bytes).
	// We wait up to 2 seconds for the host to open the port properly.
	waitForDTR(s.serial, 2*time.Second)

	for {
		// Read and process binary frames
		frame, err := protocol.ReadFrame(reader)
		if err != nil {
			// Frame error - sync byte wrong, CRC mismatch, etc.
			// Continue to next iteration to try reading again
			continue
		}

		// Process the command
		resp := s.handler.Handle(frame)

		// Send response
		if err := protocol.WriteResponse(s.serial, resp); err != nil {
			// Write error - continue and try to handle next frame
			continue
		}
	}
}

// serialReader adapts machine.Serialer to io.Reader.
// machine.Serialer provides ReadByte() but not the Read() method required by io.Reader.
type serialReader struct {
	serial machine.Serialer
}

// Read implements io.Reader by reading bytes one at a time via ReadByte.
// This blocks until len(p) bytes are available or an error occurs.
func (r *serialReader) Read(p []byte) (n int, err error) {
	for i := range p {
		b, err := r.serial.ReadByte()
		if err != nil {
			return i, err
		}
		p[i] = b
	}
	return len(p), nil
}

// Ensure serialReader implements io.Reader
var _ io.Reader = (*serialReader)(nil)
