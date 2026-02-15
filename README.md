# TinyGo TuffPad

A USB HID gamepad and keyboard firmware for RP2040 microcontrollers, written in [TinyGo](https://tinygo.org/).

## Features

- **Composite HID Device** - Simultaneous gamepad and keyboard USB HID interface
- **16-Button Gamepad** - Standard gamepad buttons with 4 analog axes (X, Y, Z, Rz)
- **USB Keyboard** - Full keyboard HID support with LED indicators
- **Serial Communication** - USB CDC serial for device detection and configuration
- **Flash Storage** - Persistent configuration storage using tinyfs
- **Dual-Core Support** - Leverages RP2040's dual-core architecture with goroutines

## Hardware

Tested on:
- [Waveshare RP2040-Zero](https://www.waveshare.com/rp2040-zero.htm)

## Building

### Requirements

- Docker (recommended) or [TinyGo 0.40.1](https://tinygo.org/getting-started/)

### Build with Docker

```bash
docker run --rm -v $(pwd):/src -w /src tinygo/tinygo:0.40.1 \
    tinygo build -o /src/waveshare-tuffpad.uf2 \
    -size=short \
    -target=waveshare-rp2040-zero .
```

### Flash

1. Hold the BOOTSEL button while connecting the RP2040 to your computer
2. Copy `waveshare-tuffpad.uf2` to the USB mass storage device that appears

## Project Structure

```
.
├── main.go                    # Entry point
├── serial/                    # USB CDC serial handler
│   └── serial.go
├── pkg/
│   ├── composite/             # USB HID descriptor
│   │   └── descriptor.go
│   ├── config/                # Configuration management
│   │   ├── config.go
│   │   └── config_test.go
│   ├── gamepad/               # HID gamepad implementation
│   │   └── gamepad.go
│   ├── keyboard/              # HID keyboard interface
│   │   └── keyboard.go
│   ├── protocol/              # Serial protocol
│   │   ├── protocol.go
│   │   └── protocol_test.go
│   └── storage/               # Flash storage (tinyfs)
│       ├── storage.go
│       └── storage_test.go
└── goroutine architecture.md  # RP2040 goroutine design notes
```

## Architecture

This project uses TinyGo's multicore scheduler to run goroutines across both RP2040 CPU cores:

- **Core 0**: Main coordinator, USB HID handling, serial commands
- **Core 1**: Input sampling, LED animations, non-blocking tasks

The USB HID implementation is non-blocking - `SendState()` copies reports to a ring buffer and returns immediately. Actual transmission happens in the USB interrupt handler.

See [goroutine architecture.md](goroutine%20architecture.md) for detailed design notes.

## Usage

### Gamepad

```go
import "github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/gamepad"

gp := gamepad.Port()
gp.Press(gamepad.ButtonA)
gp.SetAxis(gamepad.AxisX, 100)
gp.SendState()
```

### Serial Protocol

The device responds to newline-terminated commands over USB CDC serial:

```
Command:  "areyouatuffpad?"
Response: "areyouatuffpad?yes"
```

## License

MIT License - See LICENSE file for details.
