package main

import (
	"errors"
	"fmt"
	"machine"
	"time"

	pio "github.com/tinygo-org/pio/rp2-pio"
)

const (
	TicksPerRotation = 4096

	// STS3215 Instructions
	InstPing      = 0x01
	InstRead      = 0x02
	InstWrite     = 0x03
	InstRegWrite  = 0x04
	InstAction    = 0x05
	InstSyncWrite = 0x83
	InstSyncRead  = 0x82

	// STS3215 Registers
	RegID              = 0x05
	RegBaudRate        = 0x06
	RegLock            = 0x37
	RegTorqueEnable    = 0x28
	RegTargetPosition  = 0x2A
	RegTargetSpeed     = 0x2E
	RegCurrentPosition = 0x38
	RegCurrentSpeed    = 0x3A
	RegCurrentLoad     = 0x3C
	RegCurrentVoltage  = 0x3E
	RegCurrentTemp     = 0x3F
)

type STS3215 struct {
	transport *PIOTransport
	Debug     bool
}

//go:generate pioasm -o go sts3215.pio sts3215_pio.go

func NewSTS3215(transport *PIOTransport) *STS3215 {
	return &STS3215{
		transport: transport,
	}
}

func (s *STS3215) Enable(en bool) {
	s.transport.Enable(en)
}

func (s *STS3215) Close() error {
	return s.transport.Close()
}

func (s *STS3215) WriteRaw(id uint8, inst uint8, params []uint8) int {
	length := uint8(len(params) + 2)
	sum := id + length + inst
	for _, p := range params {
		sum += p
	}
	checksum := ^sum

	packet := []byte{0xFF, 0xFF, id, length, inst}
	packet = append(packet, params...)
	packet = append(packet, checksum)

	if s.Debug {
		fmt.Print("TX:")
		for _, b := range packet {
			fmt.Printf(" %02X", b)
		}
		fmt.Println()
	}

	s.transport.WritePacket(packet)
	return len(packet)
}

type PIOTransport struct {
	pin  machine.Pin
	txSm pio.StateMachine
	rxSm pio.StateMachine
}

func NewPIOTransport(pin machine.Pin) (*PIOTransport, error) {
	p := pio.PIO0
	asm := pio.AssemblerV0{}

	txSm, err := p.ClaimStateMachine()
	if err != nil {
		return nil, err
	}
	rxSm, err := p.ClaimStateMachine()
	if err != nil {
		return nil, err
	}

	txOff, err := p.AddProgram(sts3215_txInstructions, -1)
	if err != nil {
		return nil, err
	}
	rxOff, err := p.AddProgram(sts3215_rxInstructions, -1)
	if err != nil {
		return nil, err
	}

	// Configure TX
	txCfg := sts3215_txProgramDefaultConfig(txOff)
	txCfg.SetOutPins(pin, 1)
	txCfg.SetSetPins(pin, 1)
	txCfg.SetOutShift(true, false, 32)
	txCfg.SetFIFOJoin(pio.FifoJoinTx)
	{
		divWhole, divFrac, _ := pio.ClkDivFromFrequency(8000000, machine.CPUFrequency())
		txCfg.SetClkDivIntFrac(divWhole, divFrac)
	}
	txSm.Init(txOff, txCfg)
	txSm.SetPindirsConsecutive(pin, 1, true)
	txSm.Exec(asm.Set(pio.SetDestPins, 0).Encode())
	txSm.Exec(asm.Set(pio.SetDestPindirs, 0).Encode())

	// Configure RX
	rxCfg := sts3215_rxProgramDefaultConfig(rxOff)
	rxCfg.SetInPins(pin, 1)
	rxCfg.SetInShift(true, false, 8)
	rxCfg.SetFIFOJoin(pio.FifoJoinRx)
	{
		divWhole, divFrac, _ := pio.ClkDivFromFrequency(8000000, machine.CPUFrequency())
		rxCfg.SetClkDivIntFrac(divWhole, divFrac)
	}
	rxSm.Init(rxOff, rxCfg)

	pin.Configure(machine.PinConfig{Mode: machine.PinPIO0})

	return &PIOTransport{
		pin:  pin,
		txSm: txSm,
		rxSm: rxSm,
	}, nil
}

func (p *PIOTransport) WritePacket(packet []byte) {
	// Clear RX FIFO before sending
	for !p.rxSm.IsRxFIFOEmpty() {
		p.rxSm.RxGet()
	}

	for i, b := range packet {
		val := uint32(1) // Start bit
		for j := 0; j < 8; j++ {
			if (b & (1 << j)) == 0 {
				val |= (1 << (j + 1))
			}
		}
		p.txSm.TxPut(val)

		// Wait for the byte to finish shifting out and echo back (10 bits @ 1Mbps = 10us)
		// 30us ensures the byte is fully received and in the RX FIFO.
		time.Sleep(30 * time.Microsecond)

		// Drain echo for all but the last byte. The last byte's echo is left
		// in the FIFO — ReadResponse's FF FF header scan naturally skips it.
		// Draining the last echo risks eating the servo's first response byte
		// if the servo's return delay is shorter than our drain timing.
		if i < len(packet)-1 {
			for !p.rxSm.IsRxFIFOEmpty() {
				p.rxSm.RxGet()
			}
		}
	}
}

func (p *PIOTransport) Buffered() int {
	if p.rxSm.IsRxFIFOEmpty() {
		return 0
	}
	return 1
}

func (p *PIOTransport) ReadByte() (byte, error) {
	if p.rxSm.IsRxFIFOEmpty() {
		return 0, errors.New("empty")
	}
	return byte(p.rxSm.RxGet() >> 24), nil
}

func (p *PIOTransport) Enable(en bool) {
	p.txSm.SetEnabled(en)
	p.rxSm.SetEnabled(en)
}

func (p *PIOTransport) Close() error {
	p.txSm.SetEnabled(false)
	p.rxSm.SetEnabled(false)
	p.txSm.Unclaim()
	p.rxSm.Unclaim()
	return nil
}

func (s *STS3215) ReadResponse(timeout time.Duration) ([]byte, error) {
	start := time.Now()
	var resp [64]byte
	idx := 0
	state := 0 // 0: wait FF, 1: wait second FF, 2: wait ID, 3: wait LEN, 4: wait DATA
	var length uint8

	for time.Since(start) < timeout {
		if s.transport.Buffered() == 0 {
			time.Sleep(10 * time.Microsecond)
			continue
		}

		b, err := s.transport.ReadByte()
		if err != nil {
			time.Sleep(10 * time.Microsecond)
			continue
		}

		switch state {
		case 0: // Wait for first FF
			if b == 0xFF {
				state = 1
			}
		case 1: // Wait for second FF
			if b == 0xFF {
				state = 2
			} else {
				state = 0
			}
		case 2: // Wait for ID (must not be FF)
			if b != 0xFF {
				resp[0] = 0xFF
				resp[1] = 0xFF
				resp[2] = b
				idx = 3
				state = 3
			}
		case 3: // Length
			if idx < len(resp) {
				resp[idx] = b
				length = b
				idx++
				if length < 2 || length > 60 {
					state = 0
				} else {
					state = 4
				}
			} else {
				state = 0
			}
		case 4: // Data + Checksum
			if idx < len(resp) {
				resp[idx] = b
				idx++
				if idx == int(length)+4 {
					// Validate checksum
					sum := uint8(0)
					for i := 2; i < idx-1; i++ {
						sum += resp[i]
					}
					if resp[idx-1] != ^sum {
						state = 0
						continue
					}

					ret := make([]byte, idx)
					copy(ret, resp[:idx])
					if s.Debug {
						fmt.Print("RX:")
						for _, b := range ret {
							fmt.Printf(" %02X", b)
						}
						fmt.Println()
					}
					return ret, nil
				}
			} else {
				state = 0
			}
		}
	}
	return nil, errors.New("timeout")
}

func (s *STS3215) Ping(id uint8) error {
	s.WriteRaw(id, InstPing, nil)
	_, err := s.ReadResponse(time.Millisecond * 100)
	if err == nil {
		return nil
	}
	_, err = s.ReadRegister(id, RegID, 1)
	return err
}

func (s *STS3215) ReadRegister(id uint8, reg uint8, count uint8) ([]byte, error) {
	const maxRetries = 2
	var resp []byte
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		s.WriteRaw(id, InstRead, []uint8{reg, count})
		resp, err = s.ReadResponse(time.Millisecond * 100)
		if err == nil {
			break
		}
		// Brief delay before retry to let bus settle
		time.Sleep(time.Millisecond)
	}
	if err != nil {
		return nil, err
	}
	if len(resp) < 6 {
		return nil, errors.New("packet too short")
	}

	end := len(resp) - 1
	if end <= 5 {
		return nil, errors.New("no data in packet")
	}
	data := resp[5:end]
	if len(data) < int(count) {
		return nil, fmt.Errorf("short read: expected %d, got %d", count, len(data))
	}
	return data, nil
}

func (s *STS3215) WriteRegister(id uint8, reg uint8, data []uint8) {
	s.WriteRaw(id, InstWrite, append([]uint8{reg}, data...))
}

func (s *STS3215) SyncWriteRegister(ids []uint8, reg uint8, data []uint8) {
	if len(ids) == 0 {
		return
	}

	params := []uint8{reg, uint8(len(data))}
	for _, id := range ids {
		params = append(params, id)
		params = append(params, data...)
	}
	s.WriteRaw(0xFE, InstSyncWrite, params)
}

func (s *STS3215) SetID(oldID, newID uint8) error {
	if s.Debug {
		fmt.Printf("SetID: Unlocking ID %d\n", oldID)
	}
	s.WriteRegister(oldID, RegLock, []uint8{0})
	s.ReadResponse(50 * time.Millisecond) // consume servo response

	if s.Debug {
		fmt.Printf("SetID: Changing ID %d to %d\n", oldID, newID)
	}
	s.WriteRegister(oldID, RegID, []uint8{newID})
	s.ReadResponse(100 * time.Millisecond) // consume servo response

	if s.Debug {
		fmt.Printf("SetID: Locking ID %d\n", newID)
	}
	s.WriteRegister(newID, RegLock, []uint8{1})
	s.ReadResponse(50 * time.Millisecond) // consume servo response
	return nil
}

type SyncTarget struct {
	ID    uint8
	Pos   int16
	Speed int16
}

func (s *STS3215) SyncWritePositionSpeed(targets []SyncTarget) {
	if len(targets) == 0 {
		return
	}
	
	params := []uint8{RegTargetPosition, 6}
	for _, t := range targets {
		params = append(params, t.ID)
		params = append(params, uint8(t.Pos&0xFF), uint8(t.Pos>>8))
		params = append(params, 0x00, 0x00) // Time
		params = append(params, uint8(t.Speed&0xFF), uint8(t.Speed>>8))
	}
	s.WriteRaw(0xFE, InstSyncWrite, params)
}

func (s *STS3215) SetPosition(id uint8, pos int16, speed int16) error {
	val := []uint8{
		uint8(pos & 0xFF), uint8(pos >> 8),
		0x00, 0x00,
		uint8(speed & 0xFF), uint8(speed >> 8),
	}
	s.WriteRaw(id, InstWrite, append([]uint8{RegTargetPosition}, val...))
	_, err := s.ReadResponse(50 * time.Millisecond)
	if err != nil {
		return fmt.Errorf("servo %d: %w", id, err)
	}
	return nil
}

func (s *STS3215) GetPosition(id uint8) (int16, error) {
	data, err := s.ReadRegister(id, RegCurrentPosition, 2)
	if err != nil {
		return 0, err
	}
	if len(data) < 2 {
		return 0, errors.New("short data")
	}
	return int16(uint16(data[0]) | (uint16(data[1]) << 8)), nil
}

type ServoStatus struct {
	ID      uint8
	Pos     int16
	Speed   int16
	Load    int16
	Voltage uint8
	Temp    uint8
}

func (s *STS3215) GetStatus(id uint8) (ServoStatus, error) {
	data, err := s.ReadRegister(id, RegCurrentPosition, 8)
	if err != nil {
		return ServoStatus{}, err
	}
	if len(data) < 8 {
		return ServoStatus{}, errors.New("short data")
	}
	rawLoad := uint16(data[4]) | (uint16(data[5]) << 8)
	load := int16(rawLoad & 0x3FF)
	if rawLoad&(1<<10) != 0 {
		load = -load
	}

	return ServoStatus{
		ID:      id,
		Pos:     int16(uint16(data[0]) | (uint16(data[1]) << 8)),
		Speed:   int16(uint16(data[2]) | (uint16(data[3]) << 8)),
		Load:    load,
		Voltage: data[6],
		Temp:    data[7],
	}, nil
}

func (s *STS3215) SetMiddle(id uint8) {
	s.WriteRegister(id, RegTorqueEnable, []uint8{128})
}

func (s *STS3215) Dump(duration time.Duration) {
	fmt.Printf("Dumping raw bus data for %v...\n", duration)
	start := time.Now()
	for time.Since(start) < duration {
		if s.transport.Buffered() > 0 {
			b, _ := s.transport.ReadByte()
			fmt.Printf("%02X ", b)
		}
	}
	fmt.Println("\nDump complete.")
}
