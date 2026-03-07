//go:build rp

package main

import (
	"bytes"
	"fmt"
	"machine"
	"math"
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

	// Servo health thresholds
	MaxServoTemp    = 70  // degrees C — warn
	CritServoTemp   = 80  // degrees C — disable torque
	MinServoVoltage = 100 // decivolts (10.0V) — warn
	MaxServoLoad    = 900 // raw load units — warn

	MaxSpeed int16 = 3000
)

var (
	bus        *STS3215
	servoPin   = machine.GP26
	gitHash    string
	buildTime  string
	diagClient *event.EventClient
	monitorEvt *event.EventClient
)

var servoOnline = map[uint8]bool{
	ID_FEED:   false,
	ID_BEND:   false,
	ID_ROTATE: false,
}

var servoNames = map[uint8]string{
	ID_FEED:   "FEED",
	ID_BEND:   "BEND",
	ID_ROTATE: "ROTATE",
}

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
var rollerDiameter float64 = 50.0

// Per-axis position limits: [min, max] in user units (mm for FEED, degrees for BEND/ROTATE)
var axisLimits = map[uint8][2]float64{
	ID_FEED:   {-1000, 1000},
	ID_BEND:   {-180, 180},
	ID_ROTATE: {-180, 180},
}

var axisNames = map[uint8]string{
	ID_FEED:   "LINEAR",
	ID_BEND:   "BEND",
	ID_ROTATE: "ROTATE",
}

var axisUnits = map[uint8]string{
	ID_FEED:   "mm",
	ID_BEND:   "deg",
	ID_ROTATE: "deg",
}

// clampAxis clamps axis.Position to the configured limits and returns a warning string if clamped.
func clampAxis(id uint8, axis *AxisState) string {
	limits := axisLimits[id]
	if axis.Position > limits[1] {
		axis.Position = limits[1]
		return fmt.Sprintf(" (%s clamped to %.4g%s)", axisNames[id], limits[1], axisUnits[id])
	}
	if axis.Position < limits[0] {
		axis.Position = limits[0]
		return fmt.Sprintf(" (%s clamped to %.4g%s)", axisNames[id], limits[0], axisUnits[id])
	}
	return ""
}

// clampSpeed clamps speed to MaxSpeed.
func clampSpeed(speed int16) int16 {
	if speed > MaxSpeed {
		return MaxSpeed
	}
	if speed < 0 {
		speed = -speed
	}
	return speed
}

func degreesToTicks(deg float64) int16 {
	return int16(deg * TicksPerRotation / 360.0)
}

func ticksToDegrees(ticks int16) float64 {
	return float64(ticks) * 360.0 / TicksPerRotation
}

func mmToFeedDegrees(mm float64) float64 {
	circumference := math.Pi * rollerDiameter
	return (mm / circumference) * 360.0
}

func feedDegreesToMm(deg float64) float64 {
	circumference := math.Pi * rollerDiameter
	return (deg / 360.0) * circumference
}

func initAxes() {
	time.Sleep(100 * time.Millisecond)
	for id, axis := range axes {
		axis.Offset = 2048
		pos, err := bus.GetPosition(id)
		if err != nil {
			servoOnline[id] = false
			axis.Position = 0
			fmt.Printf("Servo %d (%s): no response, assuming position 0\n", id, servoNames[id])
		} else {
			servoOnline[id] = true
			deg := ticksToDegrees(pos - 2048)
			if id == ID_FEED {
				axis.Position = feedDegreesToMm(deg)
				fmt.Printf("Servo %d (%s): position %.1fmm\n", id, servoNames[id], axis.Position)
			} else {
				axis.Position = deg
				fmt.Printf("Servo %d (%s): position %.1f°\n", id, servoNames[id], axis.Position)
			}
		}
	}
}

func main() {
	initBus(servoPin)
	initAxes()

	// Check if any servos were detected at startup
	anyOnline := false
	for _, online := range servoOnline {
		if online {
			anyOnline = true
			break
		}
	}
	if !anyOnline {
		fmt.Println("WARNING: No servos detected! Check wiring and power.")
	}

	eb := event.NewEventBus()

	serials := InitSerials(eb, topic.ReceiveCmdSerial, topic.BroadcastReply, topic.BroadcastDiag, topic.BroadcastDebug)
	go RunEvery(serials.Update, 100*time.Millisecond)

	ch := command.NewCommandHandler(eb.NewEventClient("command", topic.BroadcastReply))
	ch.Event.Subscribe(topic.ReceiveCmdSerial)

	registerHandlers(ch)

	go RunEvery(ch.Update, 50*time.Millisecond)

	diagClient = eb.NewEventClient("heartbeat", topic.BroadcastDiag)
	go RunEvery(heartbeat, 15*time.Second)

	monitorEvt = eb.NewEventClient("monitor", topic.BroadcastDiag)
	go RunEvery(monitorServos, 10*time.Second)

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

func heartbeat() {
	for _, id := range []uint8{ID_FEED, ID_BEND, ID_ROTATE} {
		err := bus.Ping(id)
		wasOnline := servoOnline[id]
		nowOnline := err == nil
		servoOnline[id] = nowOnline

		if wasOnline && !nowOnline {
			fmt.Printf("Servo %d (%s): OFFLINE\n", id, servoNames[id])
			if diagClient != nil {
				diagClient.Diag("Servo %d (%s) went OFFLINE", id, servoNames[id])
			}
		} else if !wasOnline && nowOnline {
			fmt.Printf("Servo %d (%s): ONLINE\n", id, servoNames[id])
			if diagClient != nil {
				diagClient.Diag("Servo %d (%s) came ONLINE", id, servoNames[id])
			}
		}
	}
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

func monitorServos() {
	ids := []uint8{ID_FEED, ID_BEND, ID_ROTATE}
	names := map[uint8]string{ID_FEED: "LINEAR", ID_BEND: "BEND", ID_ROTATE: "ROTATE"}

	for _, id := range ids {
		st, err := bus.GetStatus(id)
		if err != nil {
			// Skip servos that timeout — bus disconnection is handled elsewhere
			continue
		}

		name := names[id]

		// Check critical temperature — disable torque immediately
		if st.Temp >= CritServoTemp {
			bus.WriteRegister(id, RegTorqueEnable, []uint8{0})
			monitorEvt.Diag("CRITICAL: %s temp %dC >= %dC — torque disabled", name, st.Temp, CritServoTemp)
			continue
		}

		// Check warning temperature
		if st.Temp >= MaxServoTemp {
			monitorEvt.Diag("WARNING: %s temp %dC >= %dC", name, st.Temp, MaxServoTemp)
		}

		// Check low voltage
		if st.Voltage < MinServoVoltage {
			monitorEvt.Diag("WARNING: %s voltage %.1fV < %.1fV", name, float64(st.Voltage)/10.0, float64(MinServoVoltage)/10.0)
		}

		// Check high load (use absolute value since load can be negative)
		load := st.Load
		if load < 0 {
			load = -load
		}
		if load > MaxServoLoad {
			monitorEvt.Diag("WARNING: %s load %d > %d", name, load, MaxServoLoad)
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
	ch.RegisterCommandHandler(handleSetRollerDiameter, "M200")
	ch.RegisterCommandHandler(handleSoftLimits, "M211")
	ch.RegisterCommandHandler(handleSetPin, "M400")
	ch.RegisterCommandHandler(handleWaitIdle, "M401")
	ch.RegisterCommandHandler(handleSaveState, "M500")
	ch.RegisterCommandHandler(handleRestoreState, "M501")
}

func handleHelp(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	fmt.Fprintf(resp, "Wirebender - Hash: %s Built: %s\n", gitHash, buildTime)
	fmt.Fprintln(resp, "Available commands:")
	fmt.Fprintf(resp, "  G0/G1 L<mm> B<deg> R<deg> S<speed>    - Move servos (speed max %d)\n", MaxSpeed)
	fmt.Fprintln(resp, "  G28 [L] [B] [R]                       - Home (return to zero)")
	fmt.Fprintln(resp, "  G90                                   - Absolute positioning mode")
	fmt.Fprintln(resp, "  G91                                   - Relative positioning mode")
	fmt.Fprintln(resp, "  G92 [L<mm>] [B<deg>] [R<deg>]         - Set position / declare zero")
	fmt.Fprintln(resp, "  M17 [L] [B] [R] / M18 [L] [B] [R]    - Enable / disable torque")
	fmt.Fprintln(resp, "  M112                                  - Emergency stop")
	fmt.Fprintln(resp, "  M114                                  - Get current positions (L in mm, B/R in degrees)")
	fmt.Fprintln(resp, "  M122                                  - Get full servo status")
	fmt.Fprintln(resp, "  M119 B<id> E<id>                      - Scan for servos in ID range")
	fmt.Fprintln(resp, "  M121 S<oldID> P<newID>                - Change servo ID")
	fmt.Fprintln(resp, "  M120                                  - Run bus diagnostics")
	fmt.Fprintln(resp, "  M200 [D<mm>]                          - Get/set feed roller diameter")
	fmt.Fprintln(resp, "  M211 [L<mm>] [B<deg>] [R<deg>]        - Get/set soft position limits (symmetric ±)")
	fmt.Fprintln(resp, "  M123 [L] [B] [R] [S<id>]              - Set middle position (calibrate to 2048)")
	fmt.Fprintln(resp, "  M400 P<pin>                           - Set/show servo bus pin")
	fmt.Fprintln(resp, "  M401                                  - Wait for all axes to reach target")
	fmt.Fprintln(resp, "  M500                                  - Print calibration state as restore command")
	fmt.Fprintln(resp, "  M501 F<n> B<n> R<n> ...               - Restore saved calibration state")
	fmt.Fprintln(resp, "  help / ?                              - Show this help")
	return nil
}

func handleMotion(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	speed := int16(500)
	var warnings string

	// First pass: parse speed
	for _, p := range params {
		if len(p) < 2 {
			continue
		}
		if p[0] == 'S' {
			val, err := strconv.ParseInt(string(p[1:]), 10, 16)
			if err != nil {
				fmt.Fprintf(resp, "WARNING: invalid S value '%s' ", p[1:])
				continue
			}
			speed = int16(val)
		}
	}
	speed = clampSpeed(speed)

	// Second pass: parse axis positions
	var errs []string
	for _, p := range params {
		if len(p) < 2 {
			continue
		}
		var id uint8
		switch p[0] {
		case 'L':
			id = ID_FEED
		case 'B':
			id = ID_BEND
		case 'R':
			id = ID_ROTATE
		default:
			continue
		}

		val, err := strconv.ParseFloat(string(p[1:]), 64)
		if err != nil {
			fmt.Fprintf(resp, "WARNING: invalid %c value '%s' ", p[0], p[1:])
			continue
		}

		if !servoOnline[id] {
			fmt.Fprintf(resp, "WARNING: %s servo offline, skipping\n", servoNames[id])
			continue
		}

		axis := axes[id]
		if relativeMode {
			axis.Position += val
		} else {
			axis.Position = val
		}

		warnings += clampAxis(id, axis)

		var posDeg float64
		if id == ID_FEED {
			posDeg = mmToFeedDegrees(axis.Position)
		} else {
			posDeg = axis.Position
		}
		absTicks := degreesToTicks(posDeg) + axis.Offset
		if err := bus.SetPosition(id, absTicks, speed); err != nil {
			errs = append(errs, fmt.Sprintf("ERROR: %s unreachable", axisNames[id]))
		}
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(resp, e)
		}
	} else {
		resp.WriteString("ok")
	}
	if warnings != "" {
		resp.WriteString(warnings)
	}
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
		case 'L':
			ids[ID_FEED] = true
		case 'B':
			ids[ID_BEND] = true
		case 'R':
			ids[ID_ROTATE] = true
		case 'S':
			if len(p) >= 2 {
				val, err := strconv.ParseInt(string(p[1:]), 10, 16)
				if err != nil {
					fmt.Fprintf(resp, "WARNING: invalid S value '%s' ", p[1:])
					continue
				}
				speed = int16(val)
			}
		}
	}
	speed = clampSpeed(speed)

	// No axes specified means all axes
	if len(ids) == 0 {
		ids = map[uint8]bool{ID_FEED: true, ID_BEND: true, ID_ROTATE: true}
	}

	var errs []string
	for id := range ids {
		if !servoOnline[id] {
			fmt.Fprintf(resp, "WARNING: %s servo offline, skipping\n", servoNames[id])
			continue
		}
		axis := axes[id]
		axis.Position = 0
		if err := bus.SetPosition(id, axis.Offset, speed); err != nil {
			errs = append(errs, fmt.Sprintf("ERROR: %s unreachable", axisNames[id]))
		}
	}

	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(resp, e)
		}
	} else {
		resp.WriteString("ok")
	}
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
		case 'L':
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
			var err error
			val, err = strconv.ParseFloat(string(p[1:]), 64)
			if err != nil {
				fmt.Fprintf(resp, "WARNING: invalid %c value '%s' ", p[0], p[1:])
				continue
			}
		}

		pos, err := bus.GetPosition(id)
		if err != nil {
			fmt.Fprintf(resp, "ERROR reading servo %d: %s\n", id, err.Error())
			continue
		}

		axis := axes[id]
		if id == ID_FEED {
			axis.Offset = pos - degreesToTicks(mmToFeedDegrees(val))
		} else {
			axis.Offset = pos - degreesToTicks(val)
		}
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
		case "L":
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
	names := []string{"LINEAR", "BEND", "ROTATE"}

	for i, id := range ids {
		pos, err := bus.GetPosition(id)
		if err != nil {
			fmt.Fprintf(resp, "%s: ERROR (%s) ", names[i], err.Error())
		} else {
			deg := ticksToDegrees(pos - axes[id].Offset)
			if id == ID_FEED {
				fmt.Fprintf(resp, "%s: %.1fmm ", names[i], feedDegreesToMm(deg))
			} else {
				fmt.Fprintf(resp, "%s: %.1f ", names[i], deg)
			}
		}
	}
	return nil
}

func handleGetStatus(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	ids := []uint8{ID_FEED, ID_BEND, ID_ROTATE}
	names := []string{"LINEAR", "BEND", "ROTATE"}

	for i, id := range ids {
		st, err := bus.GetStatus(id)
		if err != nil {
			fmt.Fprintf(resp, "%s: ERROR (%s)\n", names[i], err.Error())
		} else {
			deg := ticksToDegrees(st.Pos - axes[id].Offset)
			var displayPos float64
			var unit string
			if id == ID_FEED {
				displayPos = feedDegreesToMm(deg)
				unit = "mm"
			} else {
				displayPos = deg
				unit = "°"
			}
			fmt.Fprintf(resp, "%s: ID:%d Pos:%.1f%s Raw:%d Speed:%d Load:%d Volt:%dV Temp:%dC\n",
				names[i], st.ID, displayPos, unit, st.Pos, st.Speed, st.Load, st.Voltage/10, st.Temp)
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
	axisIDs := map[uint8]bool{}

	var rawIDs []uint8
	for _, p := range params {
		if len(p) < 1 {
			continue
		}
		switch p[0] {
		case 'L':
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
		return fmt.Errorf("Usage: M123 [L] [B] [R] [S<id>] — specify targets explicitly")
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

func handleSoftLimits(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	if len(params) == 0 {
		// Show current limits
		for _, id := range []uint8{ID_FEED, ID_BEND, ID_ROTATE} {
			limits := axisLimits[id]
			fmt.Fprintf(resp, "%s: [%.4g, %.4g] %s\n", axisNames[id], limits[0], limits[1], axisUnits[id])
		}
		return nil
	}

	for _, p := range params {
		if len(p) < 2 {
			continue
		}
		val, err := strconv.ParseFloat(string(p[1:]), 64)
		if err != nil {
			continue
		}
		if val < 0 {
			return fmt.Errorf("Limit value must be non-negative (symmetric ±)")
		}
		var id uint8
		switch p[0] {
		case 'L':
			id = ID_FEED
		case 'B':
			id = ID_BEND
		case 'R':
			id = ID_ROTATE
		default:
			continue
		}
		axisLimits[id] = [2]float64{-val, val}
		fmt.Fprintf(resp, "%s limits set to [%.4g, %.4g] %s\n", axisNames[id], -val, val, axisUnits[id])
	}
	return nil
}

func handleSetRollerDiameter(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	for _, p := range params {
		if len(p) < 2 || p[0] != 'D' {
			continue
		}
		val, err := strconv.ParseFloat(string(p[1:]), 64)
		if err != nil {
			return fmt.Errorf("Invalid diameter: %s", p[1:])
		}
		if val <= 0 {
			return fmt.Errorf("Diameter must be positive")
		}
		rollerDiameter = val
		fmt.Fprintf(resp, "Roller diameter set to %.2fmm (circumference %.2fmm)", rollerDiameter, math.Pi*rollerDiameter)
		return nil
	}
	fmt.Fprintf(resp, "Roller diameter: %.2fmm (circumference %.2fmm)", rollerDiameter, math.Pi*rollerDiameter)
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

func isAxisSettled(id uint8) bool {
	pos, err := bus.GetPosition(id)
	if err != nil {
		return false
	}
	deg := ticksToDegrees(pos - axes[id].Offset)
	var target float64
	if id == ID_FEED {
		target = mmToFeedDegrees(axes[id].Position)
	} else {
		target = axes[id].Position
	}
	diff := deg - target
	if diff < 0 {
		diff = -diff
	}
	return diff < 2.0 // 2 degree tolerance
}

func handleWaitIdle(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	const maxRetries = 50
	const retryInterval = 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		allSettled := true
		for _, id := range []uint8{ID_FEED, ID_BEND, ID_ROTATE} {
			if !isAxisSettled(id) {
				allSettled = false
				break
			}
		}
		if allSettled {
			resp.WriteString("ok")
			return nil
		}
		time.Sleep(retryInterval)
	}

	// Timeout — report current positions for debugging
	fmt.Fprint(resp, "TIMEOUT: axes still moving. ")
	handleGetPosition(ch, resp, cmd, params)
	return nil
}

// M500 — Print current calibration state as a restorable M501 command.
// Format: M501 F<offset>:<position> B<offset>:<position> R<offset>:<position> D<diameter> G<90|91>
func handleSaveState(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	modeCode := 90
	if relativeMode {
		modeCode = 91
	}
	fmt.Fprintf(resp, "M501 F%d:%.6f B%d:%.6f R%d:%.6f D%.6f G%d",
		axes[ID_FEED].Offset, axes[ID_FEED].Position,
		axes[ID_BEND].Offset, axes[ID_BEND].Position,
		axes[ID_ROTATE].Offset, axes[ID_ROTATE].Position,
		rollerDiameter, modeCode)
	return nil
}

// M501 — Restore calibration state from parameters.
// Expected: M501 F<offset>:<position> B<offset>:<position> R<offset>:<position> D<diameter> G<90|91>
func handleRestoreState(ch *command.CommandHandler, resp *bytes.Buffer, cmd string, params [][]byte) error {
	restored := 0
	for _, p := range params {
		if len(p) < 2 {
			continue
		}
		switch p[0] {
		case 'F', 'B', 'R':
			var id uint8
			switch p[0] {
			case 'F':
				id = ID_FEED
			case 'B':
				id = ID_BEND
			case 'R':
				id = ID_ROTATE
			}
			// Parse "offset:position"
			parts := bytes.SplitN(p[1:], []byte(":"), 2)
			if len(parts) != 2 {
				return fmt.Errorf("Invalid axis param %q — expected <offset>:<position>", string(p))
			}
			offset, err := strconv.ParseInt(string(parts[0]), 10, 16)
			if err != nil {
				return fmt.Errorf("Invalid offset in %q: %v", string(p), err)
			}
			position, err := strconv.ParseFloat(string(parts[1]), 64)
			if err != nil {
				return fmt.Errorf("Invalid position in %q: %v", string(p), err)
			}
			axes[id].Offset = int16(offset)
			axes[id].Position = position
			restored++
		case 'D':
			val, err := strconv.ParseFloat(string(p[1:]), 64)
			if err != nil {
				return fmt.Errorf("Invalid diameter: %s", p[1:])
			}
			if val <= 0 {
				return fmt.Errorf("Diameter must be positive")
			}
			rollerDiameter = val
			restored++
		case 'G':
			val, err := strconv.ParseInt(string(p[1:]), 10, 16)
			if err != nil {
				return fmt.Errorf("Invalid mode: %s", p[1:])
			}
			switch val {
			case 90:
				relativeMode = false
			case 91:
				relativeMode = true
			default:
				return fmt.Errorf("Invalid mode G%d — expected G90 or G91", val)
			}
			restored++
		}
	}

	if restored == 0 {
		return fmt.Errorf("Usage: M501 F<offset>:<position> B<offset>:<position> R<offset>:<position> D<diameter> G<90|91>")
	}

	// Report restored state
	modeStr := "Absolute (G90)"
	if relativeMode {
		modeStr = "Relative (G91)"
	}
	fmt.Fprintf(resp, "Restored: FEED offset=%d pos=%.2f, BEND offset=%d pos=%.2f, ROTATE offset=%d pos=%.2f, Diameter=%.2fmm, Mode=%s",
		axes[ID_FEED].Offset, axes[ID_FEED].Position,
		axes[ID_BEND].Offset, axes[ID_BEND].Position,
		axes[ID_ROTATE].Offset, axes[ID_ROTATE].Position,
		rollerDiameter, modeStr)
	return nil
}
