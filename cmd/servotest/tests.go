package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func registerTests(tc *TestContext) {
	// Group 0: Connection & Setup
	tc.Run("TestConnection", func(tc *TestContext) {
		resp, err := tc.conn.SendCommand("help")
		tc.Assert(err == nil, "help command should not error: %v", err)
		tc.Assert(strings.Contains(resp, "Available commands"), "expected 'Available commands' in help output, got: %s", resp)
	})

	tc.Run("TestInitialPosition", func(tc *TestContext) {
		tc.SendExpectOK("G92")
		tc.AssertPosition("LINEAR", 0)
		tc.AssertPosition("BEND", 0)
	})

	tc.Reset()

	// Group 1: Basic Motion
	tc.Run("TestFeedPositive", func(tc *TestContext) {
		tc.SendExpectOK("G0 L45")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 45)
	})

	tc.Run("TestFeedNegative", func(tc *TestContext) {
		tc.SendExpectOK("G0 L-45")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", -45)
	})

	tc.Run("TestBendPositive", func(tc *TestContext) {
		tc.SendExpectOK("G0 B30")
		tc.WaitForSettle()
		tc.AssertPosition("BEND", 30)
	})

	tc.Run("TestBendNegative", func(tc *TestContext) {
		tc.SendExpectOK("G0 B-30")
		tc.WaitForSettle()
		tc.AssertPosition("BEND", -30)
	})

	tc.Run("TestFeedLargeAngle", func(tc *TestContext) {
		tc.SendExpectOK("G0 L120")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 120)
	})

	tc.Reset()

	// Group 2: Bidirectional Motion
	tc.Run("TestFeedPositiveThenNegative", func(tc *TestContext) {
		tc.SendExpectOK("G0 L45")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 45)
		tc.SendExpectOK("G0 L-45")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", -45)
	})

	tc.Run("TestBendPositiveThenNegative", func(tc *TestContext) {
		tc.SendExpectOK("G0 B30")
		tc.WaitForSettle()
		tc.AssertPosition("BEND", 30)
		tc.SendExpectOK("G0 B-30")
		tc.WaitForSettle()
		tc.AssertPosition("BEND", -30)
	})

	tc.Run("TestFeedZeroCrossing", func(tc *TestContext) {
		tc.SendExpectOK("G0 L20")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 20)
		tc.SendExpectOK("G0 L-20")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", -20)
	})

	tc.Reset()

	// Group 3: Multi-Axis
	tc.Run("TestMultiAxisMove", func(tc *TestContext) {
		tc.SendExpectOK("G0 L45 B30")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 45)
		tc.AssertPosition("BEND", 30)
	})

	tc.Run("TestMultiAxisIndependent", func(tc *TestContext) {
		tc.SendExpectOK("G0 L90")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 90)
		tc.SendExpectOK("G0 B60")
		tc.WaitForSettle()
		tc.AssertPosition("BEND", 60)
		tc.AssertPosition("LINEAR", 90) // FEED should be unchanged
	})

	tc.Reset()

	// Group 4: Absolute Mode
	tc.Run("TestAbsoluteOverwrite", func(tc *TestContext) {
		tc.SendExpectOK("G0 L60")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 60)
		tc.SendExpectOK("G0 L30")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 30)
	})

	tc.Reset()

	// Group 5: Relative Mode
	tc.Run("TestRelativeIncrement", func(tc *TestContext) {
		tc.SendExpectOK("G91")
		tc.SendExpectOK("G0 L10")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 10)
		tc.SendExpectOK("G0 L10")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 20)
		tc.SendExpectOK("G0 L10")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 30)
		tc.SendExpectOK("G90") // restore
	})

	tc.Reset()

	tc.Run("TestRelativeNegative", func(tc *TestContext) {
		tc.SendExpectOK("G91")
		tc.SendExpectOK("G0 L30")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 30)
		tc.SendExpectOK("G0 L-10")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 20)
		tc.SendExpectOK("G90")
	})

	tc.Reset()

	tc.Run("TestRelativeToAbsoluteSwitch", func(tc *TestContext) {
		tc.SendExpectOK("G91")
		tc.SendExpectOK("G0 L20")
		tc.WaitForSettle()
		tc.SendExpectOK("G0 L20")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 40)
		tc.SendExpectOK("G90")
		tc.SendExpectOK("G0 L10")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 10)
	})

	tc.Reset()

	tc.Run("TestRelativeModeBend", func(tc *TestContext) {
		tc.SendExpectOK("G91")
		tc.SendExpectOK("G0 B15")
		tc.WaitForSettle()
		tc.AssertPosition("BEND", 15)
		tc.SendExpectOK("G0 B15")
		tc.WaitForSettle()
		tc.AssertPosition("BEND", 30)
		tc.SendExpectOK("G90")
	})

	tc.Reset()

	// Group 6: Homing
	tc.Run("TestHomeAll", func(tc *TestContext) {
		tc.SendExpectOK("G0 L45 B30")
		tc.WaitForSettle()
		tc.SendExpectOK("G28")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 0)
		tc.AssertPosition("BEND", 0)
	})

	tc.Run("TestHomeFeedOnly", func(tc *TestContext) {
		tc.SendExpectOK("G0 L45 B30")
		tc.WaitForSettle()
		tc.SendExpectOK("G28 L")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 0)
		tc.AssertPosition("BEND", 30)
	})

	tc.Run("TestHomeBendOnly", func(tc *TestContext) {
		tc.SendExpectOK("G0 L45 B30")
		tc.WaitForSettle()
		tc.SendExpectOK("G28 B")
		tc.WaitForSettle()
		tc.AssertPosition("BEND", 0)
		tc.AssertPosition("LINEAR", 45)
	})

	tc.Reset()

	// Group 7: Set Position (G92)
	tc.Run("TestG92ZeroAll", func(tc *TestContext) {
		tc.SendExpectOK("G0 L45")
		tc.WaitForSettle()
		tc.SendExpectOK("G92")
		tc.AssertPosition("LINEAR", 0)
	})

	tc.Run("TestG92WithValue", func(tc *TestContext) {
		tc.SendExpectOK("G0 L90")
		tc.WaitForSettle()
		tc.SendExpectOK("G92 L45")
		tc.AssertPosition("LINEAR", 45)
	})

	tc.Run("TestG92MoveFromNewZero", func(tc *TestContext) {
		tc.SendExpectOK("G92")
		tc.SendExpectOK("G0 L30")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 30)
	})

	tc.Run("TestG92PartialAxis", func(tc *TestContext) {
		tc.SendExpectOK("G0 L60 B40")
		tc.WaitForSettle()
		tc.SendExpectOK("G92 L")
		tc.AssertPosition("LINEAR", 0)
		tc.AssertPosition("BEND", 40)
	})

	tc.Run("TestG92HomeAfterReset", func(tc *TestContext) {
		tc.SendExpectOK("G92")
		tc.SendExpectOK("G0 L45")
		tc.WaitForSettle()
		tc.SendExpectOK("G28")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 0)
	})

	tc.Reset()

	// Group 8: Speed
	tc.Run("TestSpeedSlow", func(tc *TestContext) {
		tc.SendExpectOK("G0 L45 S100")
		tc.WaitForSettle()
		tc.WaitForSettle() // 2x settle for slow speed
		tc.AssertPosition("LINEAR", 45)
	})

	tc.Run("TestSpeedFast", func(tc *TestContext) {
		tc.SendExpectOK("G0 L45 S1000")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 45)
	})

	tc.Reset()

	// Group 9: M114 & M122 Reporting
	tc.Run("TestM114Format", func(tc *TestContext) {
		resp, err := tc.conn.SendCommand("M114")
		tc.Assert(err == nil, "M114 should not error: %v", err)
		positions, err := ParseM114(resp)
		tc.Assert(err == nil, "M114 parse failed: %v", err)
		_, hasFeed := positions["LINEAR"]
		_, hasBend := positions["BEND"]
		tc.Assert(hasFeed, "M114 missing FEED axis")
		tc.Assert(hasBend, "M114 missing BEND axis")
	})

	tc.Run("TestM114Fractional", func(tc *TestContext) {
		tc.SendExpectOK("G0 L22.5")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 22.5)
	})

	tc.Run("TestM122Format", func(tc *TestContext) {
		lines, err := tc.conn.SendCommandMultiline("M122")
		tc.Assert(err == nil, "M122 should not error: %v", err)
		statuses, err := ParseM122(lines)
		tc.Assert(err == nil, "M122 parse failed: %v", err)
		feed, hasFeed := statuses["LINEAR"]
		tc.Assert(hasFeed, "M122 missing FEED axis")
		tc.Assert(feed.ID == 1, "FEED ID should be 1, got %d", feed.ID)
		bend, hasBend := statuses["BEND"]
		tc.Assert(hasBend, "M122 missing BEND axis")
		tc.Assert(bend.ID == 2, "BEND ID should be 2, got %d", bend.ID)
	})

	tc.Run("TestM122RawTicks", func(tc *TestContext) {
		// Get baseline
		lines, err := tc.conn.SendCommandMultiline("M122")
		tc.Assert(err == nil, "M122 baseline error: %v", err)
		baseline, err := ParseM122(lines)
		tc.Assert(err == nil, "M122 baseline parse failed: %v", err)
		baseRaw := baseline["LINEAR"].Raw

		tc.SendExpectOK("G0 L90")
		tc.WaitForSettle()

		lines, err = tc.conn.SendCommandMultiline("M122")
		tc.Assert(err == nil, "M122 after move error: %v", err)
		after, err := ParseM122(lines)
		tc.Assert(err == nil, "M122 after move parse failed: %v", err)

		rawDelta := math.Abs(float64(after["LINEAR"].Raw - baseRaw))
		// 90 degrees = 1024 ticks, allow generous tolerance
		tc.Assert(rawDelta > 500 && rawDelta < 1500,
			"raw tick delta for 90deg should be ~1024, got %.0f (base=%d, after=%d)",
			rawDelta, baseRaw, after["LINEAR"].Raw)
	})

	tc.Run("TestM122ConsistentWithM114", func(tc *TestContext) {
		resp, err := tc.conn.SendCommand("M114")
		tc.Assert(err == nil, "M114 error: %v", err)
		m114, err := ParseM114(resp)
		tc.Assert(err == nil, "M114 parse error: %v", err)

		lines, err := tc.conn.SendCommandMultiline("M122")
		tc.Assert(err == nil, "M122 error: %v", err)
		m122, err := ParseM122(lines)
		tc.Assert(err == nil, "M122 parse error: %v", err)

		feedDiff := math.Abs(m114["LINEAR"] - m122["LINEAR"].Pos)
		tc.Assert(feedDiff < tc.tolerance,
			"FEED position mismatch: M114=%.1f M122=%.1f (diff=%.1f)",
			m114["LINEAR"], m122["LINEAR"].Pos, feedDiff)
	})

	tc.Reset()

	// Group 10: Torque
	tc.Run("TestTorqueDisableEnable", func(tc *TestContext) {
		resp, err := tc.conn.SendCommand("M18")
		tc.Assert(err == nil, "M18 error: %v", err)
		tc.Assert(resp == "ok", "M18 expected 'ok', got: %s", resp)
		resp, err = tc.conn.SendCommand("M17")
		tc.Assert(err == nil, "M17 error: %v", err)
		tc.Assert(resp == "ok", "M17 expected 'ok', got: %s", resp)
	})

	tc.Run("TestTorquePerAxis", func(tc *TestContext) {
		for _, cmd := range []string{"M18 L", "M17 L", "M18 B", "M17 B"} {
			resp, err := tc.conn.SendCommand(cmd)
			tc.Assert(err == nil, "%s error: %v", cmd, err)
			tc.Assert(resp == "ok", "%s expected 'ok', got: %s", cmd, resp)
		}
	})

	tc.Reset()

	// Group 11: Emergency Stop — run last
	tc.Run("TestEmergencyStop", func(tc *TestContext) {
		tc.SendExpectOK("G0 L45")
		tc.WaitForSettle()

		resp, err := tc.conn.SendCommand("M112")
		tc.Assert(err == nil, "M112 error: %v", err)
		tc.Assert(strings.Contains(resp, "EMERGENCY STOP"),
			"expected 'EMERGENCY STOP', got: %s", resp)

		// Recover: re-init bus + offsets
		tc.SendExpectOK("M400 P26")
		tc.WaitForSettle()
		tc.SendExpectOK("G92")

		tc.SendExpectOK("G0 L30")
		tc.WaitForSettle()
		tc.AssertPosition("LINEAR", 30)
	})
}

func (tc *TestContext) SendExpectOK(cmd string) {
	resp, err := tc.conn.SendCommand(cmd)
	if err != nil {
		tc.Fail("command %q failed: %v", cmd, err)
		return
	}
	// M400 returns multi-line, just check for no error
	if !strings.Contains(resp, "ok") && !strings.HasPrefix(cmd, "M400") {
		tc.Fail("command %q expected 'ok', got: %s", cmd, resp)
	}
}

func (tc *TestContext) AssertPosition(axis string, expected float64) {
	const maxRetries = 3
	var resp string
	var positions map[string]float64
	var actual float64
	var ok bool
	var lastErr string

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		var err error
		resp, err = tc.conn.SendCommand("M114")
		if err != nil {
			lastErr = fmt.Sprintf("M114 failed: %v", err)
			continue
		}
		positions, err = ParseM114(resp)
		if err != nil {
			lastErr = fmt.Sprintf("M114 parse failed: %v (raw: %s)", err, resp)
			continue
		}
		actual, ok = positions[axis]
		if !ok {
			lastErr = fmt.Sprintf("%s not found in M114 response: %s", axis, resp)
			continue
		}
		// Success — got a valid reading
		lastErr = ""
		break
	}

	if lastErr != "" {
		tc.Fail("%s", lastErr)
		return
	}
	diff := math.Abs(actual - expected)
	if diff > tc.tolerance {
		tc.Fail("%s position: expected %.1f, got %.1f (diff=%.1f, tolerance=%.1f)",
			axis, expected, actual, diff, tc.tolerance)
	} else if tc.conn.verbose {
		fmt.Printf("    %s: %.1f (expected %.1f, diff=%.1f)\n", axis, actual, expected, diff)
	}
}
