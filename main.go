//go:build rp

package main

import (
	"bytes"
	"fmt"
	"machine"
	"strconv"
	"time"

	"bending-rodriguez/pkg/command"
	"bending-rodriguez/pkg/event"
	"bending-rodriguez/pkg/topic"
)

const (
	ID_FEED   = 1
	ID_BEND   = 2
	ID_ROTATE = 3
)

var (
	bus       *STS3215
	servoPin  = machine.GP26
	gitHash   string
	buildTime string
)

func main() {
	initBus(servoPin)

	eb := event.NewEventBus()

	serials := InitSerials(eb, topic.ReceiveCmdSerial, topic.BroadcastReply, topic.BroadcastDiag, topic.BroadcastDebug)
	go RunEvery(serials.Update, 100*time.Millisecond)

	ch := command.NewCommandHandler(eb.NewEventClient("command", topic.BroadcastReply))
	ch.Event.Subscribe(topic.ReceiveCmdSerial)

	registerHandlers(ch)

	go RunEvery(ch.Update, 50*time.Millisecond)

	fmt.Printf("Wirebender ready. Hash: %s Built: %s\n", gitHash, buildTime)
	fmt.Printf("Current Pin: GP%d\n", servoPin)
	println("Commands: G0/G1 (F, B, R, S), M114 (Pos), M122 (Status), M119 (Scan: B<id> E<id>), M121 S<oldID> P<newID>, M400 P<pin>, M120 (Diag)")

	for {
		time.Sleep(1 * time.Second)
	}
}

func initBus(pin machine.Pin) {
	if bus != nil {
		bus.txSm.SetEnabled(false)
		bus.rxSm.SetEnabled(false)
		bus.txSm.Unclaim()
		bus.rxSm.Unclaim()
	}

	var err error
	bus, err = NewSTS3215(pin)
	if err != nil {
		fmt.Printf("Error initializing bus on GP%d: %s\n", pin, err.Error())
		return
	}
	bus.Enable(true)
	servoPin = pin
}

func RunEvery(fn func(), interval time.Duration) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			fn()
		}
	}
}

func registerHandlers(ch *command.CommandHandler) {
	ch.RegisterCommandHandler(handleMotion, "G0", "G1")
	ch.RegisterCommandHandler(handleGetPosition, "M114")
	ch.RegisterCommandHandler(handleGetStatus, "M122")
	ch.RegisterCommandHandler(handleScan, "M119")
	ch.RegisterCommandHandler(handleSetID, "M121")
	ch.RegisterCommandHandler(handleDiagnostics, "M120")
	ch.RegisterCommandHandler(handleSetPin, "M400")
}

func handleMotion(resp *bytes.Buffer, cmd string, params [][]byte) error {
	speed := int16(500) // Default speed

	for _, p := range params {
		if len(p) < 2 {
			continue
		}
		val, _ := strconv.ParseInt(string(p[1:]), 10, 16)
		switch p[0] {
		case 'S':
			speed = int16(val)
		case 'F':
			bus.SetPosition(ID_FEED, int16(val), speed)
		case 'B':
			bus.SetPosition(ID_BEND, int16(val), speed)
		case 'R':
			bus.SetPosition(ID_ROTATE, int16(val), speed)
		}
	}
	resp.WriteString("ok")
	return nil
}

func handleGetPosition(resp *bytes.Buffer, cmd string, params [][]byte) error {
	ids := []uint8{ID_FEED, ID_BEND, ID_ROTATE}
	names := []string{"FEED", "BEND", "ROTATE"}

	for i, id := range ids {
		pos, err := bus.GetPosition(id)
		if err != nil {
			fmt.Fprintf(resp, "%s: ERROR (%s) ", names[i], err.Error())
		} else {
			fmt.Fprintf(resp, "%s: %d ", names[i], pos)
		}
	}
	return nil
}

func handleGetStatus(resp *bytes.Buffer, cmd string, params [][]byte) error {
	ids := []uint8{ID_FEED, ID_BEND, ID_ROTATE}
	names := []string{"FEED", "BEND", "ROTATE"}

	for i, id := range ids {
		st, err := bus.GetStatus(id)
		if err != nil {
			fmt.Fprintf(resp, "%s: ERROR (%s)\n", names[i], err.Error())
		} else {
			fmt.Fprintf(resp, "%s: ID:%d Pos:%d Speed:%d Load:%d Volt:%dV Temp:%dC\n",
				names[i], st.ID, st.Pos, st.Speed, st.Load, st.Voltage/10, st.Temp)
		}
	}
	return nil
}

func handleScan(resp *bytes.Buffer, cmd string, params [][]byte) error {
	begin := uint8(0)
	end := uint8(253)

	for _, p := range params {
		if len(p) < 2 {
			continue
		}
		val, err := strconv.ParseUint(string(p[1:]), 10, 8)
		if err != nil {
			fmt.Fprintf(resp, "Invalid value for %c: %s\n", p[0], p[1:])
			continue
		}
		switch p[0] {
		case 'B':
			begin = uint8(val)
		case 'E':
			end = uint8(val)
		}
	}

	if begin > end {
		return fmt.Errorf("Begin ID (%d) is greater than End ID (%d)", begin, end)
	}

	fmt.Fprintf(resp, "Scanning for servos from ID %d to %d...\n", begin, end)
	found := 0
	for id := begin; ; id++ {
		err := bus.Ping(id)
		if err == nil {
			fmt.Fprintf(resp, "Found servo at ID: %d\n", id)
			found++
		}
		// Brief sleep to not overwhelm the bus
		time.Sleep(10 * time.Millisecond)

		if id == end {
			break
		}
	}
	fmt.Fprintf(resp, "Scan complete. Found %d servos.", found)
	return nil
}

func handleSetID(resp *bytes.Buffer, cmd string, params [][]byte) error {
	var oldID, newID uint8
	var oldFound, newFound bool

	for _, p := range params {
		if len(p) < 2 {
			continue
		}
		val, err := strconv.ParseUint(string(p[1:]), 10, 8)
		if err != nil {
			continue
		}
		switch p[0] {
		case 'S':
			oldID = uint8(val)
			oldFound = true
		case 'P':
			newID = uint8(val)
			newFound = true
		}
	}

	if !oldFound || !newFound {
		return fmt.Errorf("Usage: M121 S<oldID> P<newID>")
	}

	fmt.Fprintf(resp, "Changing Servo ID from %d to %d...\n", oldID, newID)
	
	bus.Debug = true
	defer func() { bus.Debug = false }()

	err := bus.SetID(oldID, newID)
	if err != nil {
		return fmt.Errorf("SetID command failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	err = bus.Ping(newID)
	if err == nil {
		fmt.Fprintf(resp, "Success! Servo now responding at ID %d", newID)
	} else {
		fmt.Fprintf(resp, "Verification FAILED. Servo not responding at ID %d", newID)
	}
	return nil
}

func handleDiagnostics(resp *bytes.Buffer, cmd string, params [][]byte) error {
	fmt.Fprintln(resp, "Starting Bus Diagnostics (ID 1)...")
	bus.Debug = true
	defer func() { bus.Debug = false }()

	fmt.Fprintln(resp, "Ping ID 1:")
	err := bus.Ping(1)
	if err != nil {
		fmt.Fprintf(resp, "Ping ID 1 FAILED: %v\n", err)
	} else {
		fmt.Fprintln(resp, "Ping ID 1 SUCCESS")
	}

	fmt.Fprintln(resp, "\nReading Position from ID 1:")
	pos, err := bus.GetPosition(1)
	if err != nil {
		fmt.Fprintf(resp, "GetPosition ID 1 FAILED: %v\n", err)
	} else {
		fmt.Fprintf(resp, "GetPosition ID 1: %d\n", pos)
	}

	return nil
}

func handleSetPin(resp *bytes.Buffer, cmd string, params [][]byte) error {
	for _, p := range params {
		if len(p) < 2 || p[0] != 'P' {
			continue
		}
		val, err := strconv.ParseInt(string(p[1:]), 10, 8)
		if err != nil {
			return fmt.Errorf("Invalid pin: %s", p[1:])
		}
		newPin := machine.Pin(val)
		fmt.Fprintf(resp, "Re-initializing bus on GP%d...\n", newPin)
		initBus(newPin)
		return nil
	}
	fmt.Fprintf(resp, "Current Servo Pin: GP%d", servoPin)
	return nil
}
