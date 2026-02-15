package serial

import (
	"machine"
)

type Serial struct {
	serial   machine.Serialer
	inIndex  int
	inBuffer [128]byte
}

func NewSerial(serial machine.Serialer) Serial {
	return Serial{
		serial:  serial,
		inIndex: 0,
	}
}

func (s *Serial) Handle() {
	for {
		in := s.read()

		if in != "" {
			s.write("areyouatuffpad?yes")
		}
	}
}

func (s *Serial) read() string {
	byte, err := s.serial.ReadByte()
	if err != nil {
		return ""
	}

	if byte == '\n' {
		in := string(s.inBuffer[:s.inIndex-1])
		s.inIndex = 0
		return in
	}

	if s.inIndex == 127 {
		s.inIndex = 0
	}

	s.inBuffer[s.inIndex] = byte
	s.inIndex = s.inIndex + 1

	return ""
}

func (s *Serial) write(out string) {
	s.serial.Write([]byte(out + "\n"))
}
