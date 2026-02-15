package main

import (
	"machine"

	"github.com/tuffrabit/tinygo-narwhal-rp2040/serial"
)

// MAIN THREAD DUTIES
//

func main() {
	serialer := machine.Serial // USB CDC Serial
	mainSerial := serial.NewSerial(serialer)

	go mainSerial.Handle()

	// Block main goroutine to keep program running
	select {}
}
