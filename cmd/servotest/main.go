package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

type TestResult struct {
	Name   string
	Passed bool
	Errors []string
}

type TestContext struct {
	conn      *ServoConn
	settle    time.Duration
	tolerance float64
	filter    string
	results   []TestResult
	current   *TestResult
}

func main() {
	port := flag.String("port", "", "serial port (default: auto-detect /dev/cu.usbmodem*)")
	timeoutMs := flag.Int("timeout", 2000, "serial read timeout in ms")
	settleMs := flag.Int("settle", 1500, "servo settle time in ms")
	tolerance := flag.Float64("tolerance", 3.0, "position tolerance in degrees")
	verbose := flag.Bool("verbose", false, "print all serial I/O")
	run := flag.String("run", "", "filter tests by substring")
	flag.Parse()

	timeout := time.Duration(*timeoutMs) * time.Millisecond

	conn, err := Open(*port, timeout, *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	tc := &TestContext{
		conn:      conn,
		settle:    time.Duration(*settleMs) * time.Millisecond,
		tolerance: *tolerance,
		filter:    *run,
	}

	registerTests(tc)

	// Summary
	fmt.Println()
	fmt.Println("========================================")
	passed := 0
	failed := 0
	for _, r := range tc.results {
		if r.Passed {
			passed++
			fmt.Printf("  PASS  %s\n", r.Name)
		} else {
			failed++
			fmt.Printf("  FAIL  %s\n", r.Name)
			for _, e := range r.Errors {
				fmt.Printf("        %s\n", e)
			}
		}
	}
	fmt.Println("========================================")
	fmt.Printf("Results: %d passed, %d failed, %d total\n", passed, failed, len(tc.results))

	if failed > 0 {
		os.Exit(1)
	}
}

func (tc *TestContext) Run(name string, fn func(tc *TestContext)) {
	if tc.filter != "" && !strings.Contains(name, tc.filter) {
		return
	}

	result := TestResult{Name: name, Passed: true}
	tc.current = &result

	fmt.Printf("--- %s\n", name)
	fn(tc)

	tc.results = append(tc.results, result)
	if result.Passed {
		fmt.Printf("    PASS\n")
	} else {
		fmt.Printf("    FAIL\n")
	}
	tc.current = nil
}

func (tc *TestContext) Assert(cond bool, format string, args ...any) {
	if !cond {
		tc.Fail(format, args...)
	}
}

func (tc *TestContext) Fail(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("    ERROR: %s\n", msg)
	if tc.current != nil {
		tc.current.Passed = false
		tc.current.Errors = append(tc.current.Errors, msg)
	}
}

func (tc *TestContext) WaitForSettle() {
	time.Sleep(tc.settle)
}

func (tc *TestContext) Reset() {
	fmt.Println("--- [reset]")
	tc.SendExpectOK("G90")
	tc.SendExpectOK("G92")
	tc.SendExpectOK("G28")
	tc.WaitForSettle()
}

func (tc *TestContext) GetStatus() map[string]*AxisStatus {
	lines, err := tc.conn.SendCommandMultiline("M122")
	if err != nil {
		tc.Fail("M122 failed: %v", err)
		return nil
	}
	statuses, err := ParseM122(lines)
	if err != nil {
		tc.Fail("M122 parse failed: %v", err)
		return nil
	}
	return statuses
}
