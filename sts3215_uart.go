package main

import (
	"errors"
	"machine"
	"time"
)

type UARTTransport struct {
	uart *machine.UART
}

// NewUARTTransport creates a new transport using a standard hardware UART.
// tx and rx pins should be configured appropriately before or by the UART.
// Note: It assumes the external board handles the single-wire half-duplex direction.
func NewUARTTransport(uart *machine.UART, tx, rx machine.Pin, baud uint32) (*UARTTransport, error) {
	err := uart.Configure(machine.UARTConfig{
		BaudRate: baud,
		TX:       tx,
		RX:       rx,
	})
	if err != nil {
		return nil, err
	}
	return &UARTTransport{
		uart: uart,
	}, nil
}

func (u *UARTTransport) WritePacket(packet []byte) {
	// Clear the RX buffer before sending
	for u.uart.Buffered() > 0 {
		u.uart.ReadByte()
	}

	u.uart.Write(packet)

	// Since we don't control the echo delay like in PIO, we'll give the bus a moment
	// and read back the echoed packet if the hardware doesn't suppress it.
	// Many RS485/half-duplex converters echo what they send.
	time.Sleep(100 * time.Microsecond)
	
	// Read back the echo if present. The length of the echo will be exactly len(packet) bytes, 
	// assuming it perfectly loops back. But we'll just drain whatever is in the buffer quickly.
	for i := 0; i < len(packet); i++ {
		if u.uart.Buffered() > 0 {
			u.uart.ReadByte()
		}
	}
}

func (u *UARTTransport) Buffered() int {
	return u.uart.Buffered()
}

func (u *UARTTransport) ReadByte() (byte, error) {
	if u.uart.Buffered() == 0 {
		return 0, errors.New("empty")
	}
	return u.uart.ReadByte()
}

func (u *UARTTransport) Enable(en bool) {
	// Not all UART implementations support enable/disable easily, 
	// and external boards might just work based on TX activity.
}

func (u *UARTTransport) Close() error {
	// machine.UART does not have a Close() method in standard TinyGo
	return nil
}
