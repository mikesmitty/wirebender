# Issue: Servo Bus Timeouts

## Problem

STS3215 servos intermittently fail to respond to position queries. M114 returns `FEED: ERROR (timeout)` instead of a position value. Affects all axes non-deterministically. When servos do respond, values are accurate.

## Root Causes Found

Two regressions were introduced when the code was refactored from the original version (preserved in `deleteme/`). Both directly cause intermittent timeouts.

### 1. PIO RX sampling at bit boundary instead of center (`sts3215.pio`)

The PIO RX program determines when each bit is sampled relative to the UART start bit edge. The delay between detecting the start bit and reading the first data bit is critical.

**Timing analysis** (8 cycles per bit at 8 MHz PIO clock = 1 Mbps):

```
Start bit edge detected by: wait 0 pin 0  (cycle 0)
Delay instruction:           nop [D]        (cycles 1 to 1+D)
Setup:                       set x, 7       (cycle 2+D)
First sample:                in pins, 1     (cycle 3+D)

Ideal: sample at center of bit 0 = 1.5 bit periods = cycle 12
  → need D = 9 → nop [9]
```

| Version | Instruction | First sample | Bit position | Quality |
|---------|------------|-------------|-------------|---------|
| Old (`deleteme/`) | `nop [10]` | cycle 13 | 1.625 bit periods | Good (slightly late) |
| **Broken (current)** | `nop [5]` | cycle 8 | **1.0 bit periods** | **At bit boundary** |
| **Fixed** | `nop [9]` | cycle 12 | 1.5 bit periods | Ideal center |

With `nop [5]`, sampling occurs right at the transition between the start bit and data bit 0. On an open-drain bus with slow pull-up rise times, the signal hasn't settled yet. Any jitter causes bit errors. A single corrupted byte causes the `ReadResponse` state machine to lose sync and waste the entire 100ms timeout searching for the `FF FF` header.

### 2. Echo drain eating servo response bytes (`sts3215.go:WriteRaw`)

The STS3215 bus is half-duplex on a single pin. When we transmit, the RX PIO also captures our own bytes as "echoes" that must be drained before reading the servo's response.

**The bug:** After the LAST TX byte, the code sleeps 30µs then drains ALL bytes from the RX FIFO:

```go
// BROKEN: drains echo AND response bytes
time.Sleep(30 * time.Microsecond)
for !s.rxSm.IsRxFIFOEmpty() {
    s.rxSm.RxGet() // Drain echo
}
```

For a 7-byte read command, the last byte finishes transmitting at ~190µs. With the per-byte 30µs sleep, the drain runs at ~210µs. If the servo's return delay is < 20µs, its first response byte is already in the RX FIFO. The drain loop eats it. `ReadResponse` then waits for a response that's already been discarded → timeout.

**The old code** in `deleteme/` correctly skipped the drain on the last byte:
```go
if i < len(packet)-1 {
    for !s.rxSm.IsRxFIFOEmpty() {
        s.rxSm.RxGet()
    }
}
```

The undrained last echo byte is harmless — `ReadResponse` looks for `FF FF` header bytes and naturally skips any non-header byte. Even if the echo IS `0xFF` (checksum happened to be 0xFF), the state machine correctly handles consecutive `0xFF` bytes by staying in its header-scan state until a non-`0xFF` ID byte appears.

## Fixes Applied

### 1. PIO RX: Center-of-bit sampling (`sts3215.pio`, `sts3215_pio.go`)

Changed `nop [5]` → `nop [9]` for ideal 1.5-bit-period sampling. This places the sample point at the center of each data bit, maximizing noise margin and tolerance for slow open-drain rise times.

Instruction encoding: `0xa542` → `0xa942` (delay field bits [12:8] changed from 00101 to 01001).

### 2. Echo drain: Skip last byte (`sts3215.go:WriteRaw`)

Restored the old behavior: drain echoes for all bytes except the last. The last echo is left in the RX FIFO for `ReadResponse` to skip naturally.

### 3. Retry in `ReadRegister` (`sts3215.go`)

Added automatic retry (up to 2 attempts with 1ms settle delay) at the protocol level. Even with the PIO and echo fixes, retries provide defense-in-depth against genuine hardware transients (noise, power glitches).

### 4. Retry in `AssertPosition` (`cmd/servotest/tests.go`)

Added retry logic (up to 3 attempts, 200ms between) in the test tool for end-to-end resilience.

## Communication Flow

```
handleGetPosition
  → GetPosition(id)
    → ReadRegister(id, RegCurrentPosition, 2)   [up to 2 attempts]
      → WriteRaw(id, InstRead, [reg, count])
        1. Clear RX FIFO
        2. For each byte in packet:
           a. Encode as 10-bit open-drain word (start + inverted data + stop)
           b. TxPut to PIO TX FIFO
           c. Sleep 30µs for echo
           d. Drain echo (SKIP for last byte)  ← FIX
      → ReadResponse(100ms timeout)
        1. Poll RX FIFO (10µs sleep between polls)
        2. State machine: wait FF → wait FF → read ID → read LEN → read DATA
        3. Return complete packet or timeout error
    → Extract position bytes from response
  → Convert ticks to degrees, format output
```

## Key Timing Values

| Parameter | Value | Location |
|-----------|-------|----------|
| PIO clock | 8 MHz | `sts3215.go:NewSTS3215` |
| Baud rate | 1 Mbps (8 cyc/bit) | Derived from PIO clock |
| RX sample point | 1.5 bit periods | `sts3215.pio` (was 1.0, fixed to 1.5) |
| Per-byte TX delay | 30µs | `sts3215.go:WriteRaw` |
| Response timeout | 100ms | `sts3215.go:ReadResponse` |
| RX poll interval | 10µs | `sts3215.go:ReadResponse` |
| Retry delay | 1ms | `sts3215.go:ReadRegister` |

## Remaining Considerations

- **Requires reflash** — PIO changes are compiled into firmware
- If timeouts persist after this fix, investigate hardware (wiring, pull-up resistor value, power supply)
- The `ReadResponse` polling loop sleeps 10µs between checks; on a 14-byte response at 1Mbps, the 8-deep RX FIFO provides sufficient buffering
- The retry in `ReadRegister` adds at most 101ms (100ms timeout + 1ms settle) per failed attempt — acceptable for interactive use but may need tuning for real-time motion
