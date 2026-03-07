package serial

import (
	"bufio"
	"machine"
	"os"
	"time"
)

var _ Serialer = (*PseudoSerial)(nil)

type PseudoSerial struct {
	buf *bufio.Reader
}

func NewPseudoSerial() *PseudoSerial {
	p := &PseudoSerial{
		buf: bufio.NewReader(os.Stdin),
	}
	// Check for new data on a loop so we don't hang on Buffered() calls.
	go p.peeker()

	return p
}

func (p *PseudoSerial) peeker() {
	for {
		p.buf.Peek(32)
		time.Sleep(100 * time.Millisecond)
	}
}

func (p *PseudoSerial) Buffered() int {
	return p.buf.Buffered()
}

func (p *PseudoSerial) ReadByte() (byte, error) {
	return p.buf.ReadByte()
}

func (p *PseudoSerial) WriteByte(c byte) error {
	_, err := os.Stdout.Write([]byte{c})
	return err
}

func (p *PseudoSerial) Write(data []byte) (n int, err error) {
	return os.Stdout.Write(data)
}

func (p *PseudoSerial) Configure(config machine.UARTConfig) error {
	// PseudoSerial does not require configuration
	return nil
}
