package main

import (
	"machine"

	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/display"
	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/protocol"
	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/storage"
	"github.com/tuffrabit/tinygo-narwhal-rp2040/serial"
)

func main() {
	// Initialize SSD1306 display for debug output
	// This is optional - if it fails, serial still works
	displayMgr := display.NewManager()

	// Initialize storage with on-chip flash
	// Format=true allows automatic formatting on first boot
	storageMgr, err := storage.New(machine.Flash, true)
	if err != nil {
		// Storage init failure is critical - flash LED or log if possible
		// For now, continue anyway so serial still works for recovery
		if displayMgr != nil {
			displayMgr.ShowError("Storage init failed")
		}
	}

	// Create protocol handler with storage
	protoHandler := protocol.NewHandler(storageMgr)

	// Create serial handler with protocol
	serialer := machine.Serial // USB CDC Serial
	mainSerial := serial.NewSerial(serialer, protoHandler)

	// Connect display to serial handler for debug output
	mainSerial.SetDisplay(displayMgr)

	// Start serial handling in its own goroutine
	go mainSerial.Handle()

	// Block main goroutine to keep program running
	select {}
}
