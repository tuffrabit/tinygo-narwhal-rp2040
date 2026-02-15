package keyboard

import (
	tgk "machine/usb/hid/keyboard"
)

type Keyboard interface {
	TxHandler() bool
	RxHandler(b []byte) bool
	NumLockLed() bool
	CapsLockLed() bool
	ScrollLockLed() bool
	Write(b []byte) (n int, err error)
	WriteByte(b byte) error
	Press(c tgk.Keycode) error
	Down(c tgk.Keycode) error
	Up(c tgk.Keycode) error
	Release() error
}
