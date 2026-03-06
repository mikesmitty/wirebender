package serial

import (
	"bytes"
	"machine"

	"bending-rodriguez/pkg/event"
)

type Serial struct {
	buf  *bytes.Buffer
	uart Serialer

	Event *event.EventClient
}

type Serialer interface {
	WriteByte(c byte) error
	Write(data []byte) (n int, err error)
	Buffered() int
	ReadByte() (byte, error)
	Configure(config machine.UARTConfig) error
}

func NewSerial(uart Serialer, cl *event.EventClient) *Serial {
	s := &Serial{
		buf:  new(bytes.Buffer),
		uart: uart,

		Event: cl,
	}

	if s.uart == nil {
		panic("UART not configured")
	}

	return s
}

func (s *Serial) Update() {
	select {
	case evt := <-s.Event.Receive:
		switch msg := evt.Data.(type) {
		case string:
			s.uart.Write([]byte(msg))
		case *bytes.Buffer:
			s.uart.Write(msg.Bytes())
		default:
			s.Event.Debug("Serial received unknown data type: %T", evt.Data)
			return
		}
		s.uart.Write([]byte("\n"))

	default:
		s.ReadCommand()
	}
}

func (s *Serial) ReadCommand() {
	if s.uart == nil {
		return
	}

	for s.uart.Buffered() > 0 {
		b, err := s.uart.ReadByte()
		if err == nil {
			s.buf.WriteByte(b)
		}
	}

	for s.buf.Len() > 0 {
		data := s.buf.Bytes()

		start := bytes.IndexByte(data, '<')
		if start == -1 {
			// No start char '<' found, clear the cmdBuffer
			s.buf.Reset()
			break
		}

		if start > 0 {
			// Overwrite any junk data before the start of the command
			s.buf.Next(start)
			data = s.buf.Bytes()
		}

		end := bytes.IndexByte(data, '>')
		if end == -1 {
			// We have an incomplete command. Wait for more data.
			break
		}

		// Command is trimmed of the start '<' and end '>' before sending
		cmdBuf := new(bytes.Buffer)
		command := data[1:end]
		cmdBuf.Write(command)
		s.Event.Publish(cmdBuf)

		// Reset to break the loop and be ready to start again
		s.buf.Reset()
	}
}
