package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.bug.st/serial"
)

type ServoConn struct {
	port    serial.Port
	timeout time.Duration
	verbose bool
}

func Open(portName string, timeout time.Duration, verbose bool) (*ServoConn, error) {
	if portName == "" {
		var err error
		portName, err = detectPort()
		if err != nil {
			return nil, err
		}
	}

	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", portName, err)
	}

	port.SetReadTimeout(timeout)

	conn := &ServoConn{
		port:    port,
		timeout: timeout,
		verbose: verbose,
	}

	// Flush any stale data
	conn.Flush()

	fmt.Printf("Connected to %s\n", portName)
	return conn, nil
}

func detectPort() (string, error) {
	matches, _ := filepath.Glob("/dev/cu.usbmodem*")
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no USB serial ports found (tried /dev/cu.usbmodem*)")
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple USB serial ports found: %s — use -port to specify", strings.Join(matches, ", "))
	}
}

func (c *ServoConn) SendCommand(cmd string) (string, error) {
	c.Flush()

	if c.verbose {
		fmt.Printf("  >> %s\n", cmd)
	}

	_, err := c.port.Write([]byte(cmd + "\r\n"))
	if err != nil {
		return "", fmt.Errorf("write: %w", err)
	}

	var all []byte
	buf := make([]byte, 4096)
	deadline := time.Now().Add(c.timeout)
	for time.Now().Before(deadline) {
		n, _ := c.port.Read(buf)
		if n > 0 {
			all = append(all, buf[:n]...)
			if bytes.Contains(all, []byte("\r\n")) {
				break
			}
		}
	}

	resp := strings.TrimSpace(string(all))
	if c.verbose {
		fmt.Printf("  << %s\n", resp)
	}
	return resp, nil
}

func (c *ServoConn) SendCommandMultiline(cmd string) ([]string, error) {
	c.Flush()

	if c.verbose {
		fmt.Printf("  >> %s\n", cmd)
	}

	_, err := c.port.Write([]byte(cmd + "\r\n"))
	if err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Read with gap-based termination for multi-line responses
	var all []byte
	buf := make([]byte, 4096)
	c.port.SetReadTimeout(c.timeout)

	for {
		n, err := c.port.Read(buf)
		if n > 0 {
			all = append(all, buf[:n]...)
		}
		if err != nil || n == 0 {
			break
		}
		// Brief pause then check for more data
		c.port.SetReadTimeout(200 * time.Millisecond)
	}

	// Restore original timeout
	c.port.SetReadTimeout(c.timeout)

	raw := strings.TrimSpace(string(all))
	if c.verbose {
		fmt.Printf("  << %s\n", raw)
	}

	if raw == "" {
		return nil, nil
	}

	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func (c *ServoConn) Flush() {
	buf := make([]byte, 4096)
	c.port.SetReadTimeout(50 * time.Millisecond)
	for {
		n, _ := c.port.Read(buf)
		if n == 0 {
			break
		}
	}
	c.port.SetReadTimeout(c.timeout)
}

func (c *ServoConn) Close() {
	if c.port != nil {
		c.port.Close()
	}
}
