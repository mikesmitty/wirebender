//go:build rp

package serial

import (
	"bytes"
	"machine"

	"github.com/mikesmitty/wirebender/pkg/event"
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
		s.uart.Write([]byte("\r\n"))

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

		// Support <...> format
		start := bytes.IndexByte(data, '<')
		if start != -1 {
			if start > 0 {
				s.buf.Next(start)
				data = s.buf.Bytes()
			}

			end := bytes.IndexByte(data, '>')
			if end != -1 {
				cmdBuf := new(bytes.Buffer)
				command := data[1:end]
				cmdBuf.Write(command)
				s.Event.Publish(cmdBuf)
				s.buf.Next(end + 1)
				continue
			}
			// Incomplete <... command, wait for more data
			break
		}

		// Support line-based format (G-code style)
		lineEnd := bytes.IndexAny(data, "\r\n")
		if lineEnd != -1 {
			if lineEnd > 0 {
				cmdBuf := new(bytes.Buffer)
				command := data[:lineEnd]
				cmdBuf.Write(command)
				s.Event.Publish(cmdBuf)
			}
			// Consume the command and the line ending
			s.buf.Next(lineEnd + 1)
			// Consume any following \r or \n
			for s.buf.Len() > 0 {
				next := s.buf.Bytes()[0]
				if next == '\r' || next == '\n' {
					s.buf.ReadByte()
				} else {
					break
				}
			}
			continue
		}

		// No < and no \r\n found.
		// If the buffer is getting too large without a terminator, clear it
		if s.buf.Len() > 128 {
			s.buf.Reset()
		}
		break
	}
}
