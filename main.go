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

type AxisState struct {
	Offset   int16
	Position float64
}

var axes = map[uint8]*AxisState{
	ID_FEED:   {},
	ID_BEND:   {},
	ID_ROTATE: {},
}
var relativeMode = false

func degreesToTicks(deg float64) int16 {
	return int16(deg * TicksPerRotation / 360.0)
}

func ticksToDegrees(ticks int16) float64 {
	return float64(ticks) * 360.0 / TicksPerRotation
}

func initAxes() {
	time.Sleep(100 * time.Millisecond) // let servos boot
	for id, axis := range axes {
		pos, err := bus.GetPosition(id)
		if err != nil {
			axis.Offset = 2048 // fallback to servo center
			fmt.Printf("Servo %d: no response, defaulting offset to 2048\n", id)
		} else {
			axis.Offset = pos
			fmt.Printf("Servo %d: position %d, offset set\n", id, pos)
		}
		axis.Position = 0
	}
}

func main() {
	initBus(servoPin)
	initAxes()

	eb := event.NewEventBus()

	serials := InitSerials(eb, topic.ReceiveCmdSerial, topic.BroadcastReply, topic.BroadcastDiag, topic.BroadcastDebug)
	go RunEvery(serials.Update, 100*time.Millisecond)

	ch := command.NewCommandHandler(eb.NewEventClient("command", topic.BroadcastReply))
	ch.Event.Subscribe(topic.ReceiveCmdSerial)

	registerHandlers(ch)

	go RunEvery(ch.Update, 50*time.Millisecond)

	fmt.Printf("Wirebender ready. Hash: %s Built: %s\n", gitHash, buildTime)
	fmt.Printf("Current Pin: GP%d\n", servoPin)
	fmt.Println("Mode: Absolute (G90)")
	println("Type 'help' or '?' for available commands.")

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
	ch.RegisterCommandHandler(handleHelp, "help", "?")
	ch.RegisterCommandHandler(handleMotion, "G0", "G1")
	ch.RegisterCommandHandler(handleHome, "G28")
	ch.RegisterCommandHandler(handleSetAbsolute, "G90")
	ch.RegisterCommandHandler(handleSetRelative, "G91")
	ch.RegisterCommandHandler(handleSetPosition, "G92")
	ch.RegisterCommandHandler(handleTorqueEnable, "M17")
	ch.RegisterCommandHandler(handleTorqueDisable, "M18")
	ch.RegisterCommandHandler(handleEmergencyStop, "M112")
	ch.RegisterCommandHandler(handleGetPosition, "M114")
	ch.RegisterCommandHandler(handleGetStatus, "M122")
	ch.RegisterCommandHandler(handleScan, "M119")
	ch.RegisterCommandHandler(handleSetID, "M121")
	ch.RegisterCommandHandler(handleDiagnostics, "M120")
	ch.RegisterCommandHandler(handleSetMiddle, "M123")
	ch.RegisterCommandHandler(handleSetPin, "M400")
}

func handleHelp(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	fmt.Fprintf(resp, "Wirebender - Hash: %s Built: %s\n", gitHash, buildTime)
	fmt.Fprintln(resp, "Available commands:")
	fmt.Fprintln(resp, "  G0/G1 F<deg> B<deg> R<deg> S<speed>   - Move servos (degrees, speed in raw units)")
	fmt.Fprintln(resp, "  G28 [F] [B] [R]                       - Home (return to zero)")
	fmt.Fprintln(resp, "  G90                                   - Absolute positioning mode")
	fmt.Fprintln(resp, "  G91                                   - Relative positioning mode")
	fmt.Fprintln(resp, "  G92 [F<deg>] [B<deg>] [R<deg>]        - Set position / declare zero")
	fmt.Fprintln(resp, "  M17 [F] [B] [R] / M18 [F] [B] [R]    - Enable / disable torque")
	fmt.Fprintln(resp, "  M112                                  - Emergency stop")
	fmt.Fprintln(resp, "  M114                                  - Get current positions (degrees)")
	fmt.Fprintln(resp, "  M122                                  - Get full servo status")
	fmt.Fprintln(resp, "  M119 B<id> E<id>                      - Scan for servos in ID range")
	fmt.Fprintln(resp, "  M121 S<oldID> P<newID>                - Change servo ID")
	fmt.Fprintln(resp, "  M120                                  - Run bus diagnostics")
	fmt.Fprintln(resp, "  M123 [F] [B] [R] [S<id>]              - Set middle position (calibrate to 2048)")
	fmt.Fprintln(resp, "  M400 P<pin>                           - Set/show servo bus pin")
	fmt.Fprintln(resp, "  help / ?                              - Show this help")
	return nil
}

func handleMotion(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	speed := int16(500)

	// First pass: parse speed
	for _, p := range params {
		if len(p) < 2 {
			continue
		}
		if p[0] == 'S' {
			val, _ := strconv.ParseInt(string(p[1:]), 10, 16)
			speed = int16(val)
		}
	}

	// Second pass: parse axis positions
	for _, p := range params {
		if len(p) < 2 {
			continue
		}
		var id uint8
		switch p[0] {
		case 'F':
			id = ID_FEED
		case 'B':
			id = ID_BEND
		case 'R':
			id = ID_ROTATE
		default:
			continue
		}

		deg, err := strconv.ParseFloat(string(p[1:]), 64)
		if err != nil {
			continue
		}

		axis := axes[id]
		if relativeMode {
			axis.Position += deg
		} else {
			axis.Position = deg
		}

		absTicks := degreesToTicks(axis.Position) + axis.Offset
		bus.SetPosition(id, absTicks, speed)
	}
	resp.WriteString("ok")
	return nil
}

func handleHome(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	speed := int16(500)

	ids := map[uint8]bool{}
	for _, p := range params {
		if len(p) < 1 {
			continue
		}
		switch p[0] {
		case 'F':
			ids[ID_FEED] = true
		case 'B':
			ids[ID_BEND] = true
		case 'R':
			ids[ID_ROTATE] = true
		case 'S':
			if len(p) >= 2 {
				val, _ := strconv.ParseInt(string(p[1:]), 10, 16)
				speed = int16(val)
			}
		}
	}

	// No axes specified means all axes
	if len(ids) == 0 {
		ids = map[uint8]bool{ID_FEED: true, ID_BEND: true, ID_ROTATE: true}
	}

	for id := range ids {
		axis := axes[id]
		axis.Position = 0
		bus.SetPosition(id, axis.Offset, speed)
	}

	resp.WriteString("ok")
	return nil
}

func handleSetAbsolute(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	relativeMode = false
	resp.WriteString("ok")
	return nil
}

func handleSetRelative(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	relativeMode = true
	resp.WriteString("ok")
	return nil
}

func handleSetPosition(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	if len(params) == 0 {
		// Zero all axes
		for id, axis := range axes {
			pos, err := bus.GetPosition(id)
			if err != nil {
				fmt.Fprintf(resp, "ERROR reading servo %d: %s\n", id, err.Error())
				continue
			}
			axis.Offset = pos
			axis.Position = 0
		}
		resp.WriteString("ok")
		return nil
	}

	for _, p := range params {
		if len(p) < 1 {
			continue
		}
		var id uint8
		switch p[0] {
		case 'F':
			id = ID_FEED
		case 'B':
			id = ID_BEND
		case 'R':
			id = ID_ROTATE
		default:
			continue
		}

		val := 0.0
		if len(p) >= 2 {
			val, _ = strconv.ParseFloat(string(p[1:]), 64)
		}

		pos, err := bus.GetPosition(id)
		if err != nil {
			fmt.Fprintf(resp, "ERROR reading servo %d: %s\n", id, err.Error())
			continue
		}

		axis := axes[id]
		axis.Offset = pos - degreesToTicks(val)
		axis.Position = val
	}

	resp.WriteString("ok")
	return nil
}

func handleEmergencyStop(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	for _, id := range []uint8{ID_FEED, ID_BEND, ID_ROTATE} {
		bus.WriteRegister(id, RegTorqueEnable, []uint8{0})
	}
	bus.Enable(false)
	resp.WriteString("EMERGENCY STOP")
	return nil
}

func parseTorqueTargets(params [][]byte) []uint8 {
	var ids []uint8
	for _, p := range params {
		switch string(p) {
		case "F":
			ids = append(ids, ID_FEED)
		case "B":
			ids = append(ids, ID_BEND)
		case "R":
			ids = append(ids, ID_ROTATE)
		}
	}
	if len(ids) == 0 {
		ids = []uint8{ID_FEED, ID_BEND, ID_ROTATE}
	}
	return ids
}

func handleTorqueEnable(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	for _, id := range parseTorqueTargets(params) {
		bus.WriteRegister(id, RegTorqueEnable, []uint8{1})
	}
	resp.WriteString("ok")
	return nil
}

func handleTorqueDisable(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	for _, id := range parseTorqueTargets(params) {
		bus.WriteRegister(id, RegTorqueEnable, []uint8{0})
	}
	resp.WriteString("ok")
	return nil
}

func handleGetPosition(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	ids := []uint8{ID_FEED, ID_BEND, ID_ROTATE}
	names := []string{"FEED", "BEND", "ROTATE"}

	for i, id := range ids {
		pos, err := bus.GetPosition(id)
		if err != nil {
			fmt.Fprintf(resp, "%s: ERROR (%s) ", names[i], err.Error())
		} else {
			deg := ticksToDegrees(pos - axes[id].Offset)
			fmt.Fprintf(resp, "%s: %.1f ", names[i], deg)
		}
	}
	return nil
}

func handleGetStatus(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	ids := []uint8{ID_FEED, ID_BEND, ID_ROTATE}
	names := []string{"FEED", "BEND", "ROTATE"}

	for i, id := range ids {
		st, err := bus.GetStatus(id)
		if err != nil {
			fmt.Fprintf(resp, "%s: ERROR (%s)\n", names[i], err.Error())
		} else {
			deg := ticksToDegrees(st.Pos - axes[id].Offset)
			fmt.Fprintf(resp, "%s: ID:%d Pos:%.1f Raw:%d Speed:%d Load:%d Volt:%dV Temp:%dC\n",
				names[i], st.ID, deg, st.Pos, st.Speed, st.Load, st.Voltage/10, st.Temp)
		}
	}
	return nil
}

func handleScan(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
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

	go func() {
		buf := new(bytes.Buffer)
		fmt.Fprintf(buf, "Scanning for servos from ID %d to %d...", begin, end)
		ch.Event.Publish(buf)

		found := 0
		for id := begin; ; id++ {
			err := bus.Ping(id)
			if err == nil {
				buf = new(bytes.Buffer)
				fmt.Fprintf(buf, "Found servo at ID: %d", id)
				ch.Event.Publish(buf)
				found++
			}
			// Brief sleep to not overwhelm the bus
			time.Sleep(10 * time.Millisecond)

			if id == end {
				break
			}
		}
		buf = new(bytes.Buffer)
		fmt.Fprintf(buf, "Scan complete. Found %d servos.", found)
		ch.Event.Publish(buf)
	}()

	return nil
}

func handleSetID(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
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

func handleDiagnostics(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
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

func handleSetMiddle(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	axisNames := map[uint8]string{ID_FEED: "FEED", ID_BEND: "BEND", ID_ROTATE: "ROTATE"}
	axisIDs := map[uint8]bool{}

	var rawIDs []uint8
	for _, p := range params {
		if len(p) < 1 {
			continue
		}
		switch p[0] {
		case 'F':
			axisIDs[ID_FEED] = true
		case 'B':
			axisIDs[ID_BEND] = true
		case 'R':
			axisIDs[ID_ROTATE] = true
		case 'S':
			if len(p) >= 2 {
				val, err := strconv.ParseUint(string(p[1:]), 10, 8)
				if err == nil {
					rawIDs = append(rawIDs, uint8(val))
				}
			}
		}
	}

	if len(axisIDs) == 0 && len(rawIDs) == 0 {
		return fmt.Errorf("Usage: M123 [F] [B] [R] [S<id>] — specify targets explicitly")
	}

	// Calibrate named axes
	for _, id := range []uint8{ID_FEED, ID_BEND, ID_ROTATE} {
		if !axisIDs[id] {
			continue
		}
		bus.SetMiddle(id)
		time.Sleep(200 * time.Millisecond)
		pos, err := bus.GetPosition(id)
		if err != nil {
			fmt.Fprintf(resp, "%s (ID %d): calibrated, verify failed: %s\n", axisNames[id], id, err.Error())
		} else {
			fmt.Fprintf(resp, "%s (ID %d): calibrated, position now %d\n", axisNames[id], id, pos)
		}
		axes[id].Offset = 0
		axes[id].Position = 0
	}

	// Calibrate raw servo IDs
	for _, id := range rawIDs {
		bus.SetMiddle(id)
		time.Sleep(200 * time.Millisecond)
		pos, err := bus.GetPosition(id)
		if err != nil {
			fmt.Fprintf(resp, "Servo %d: calibrated, verify failed: %s\n", id, err.Error())
		} else {
			fmt.Fprintf(resp, "Servo %d: calibrated, position now %d\n", id, pos)
		}
	}

	return nil
}

func handleSetPin(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
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
		initAxes()
		return nil
	}
	fmt.Fprintf(resp, "Current Servo Pin: GP%d", servoPin)
	return nil
}
