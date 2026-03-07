# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Wirebender is a TinyGo firmware for a wire bending machine powered by a Raspberry Pi Pico 2 (RP2350). It controls STS3215 serial bus servos via a G-code-like command interface over USB serial. The module name is `bending-rodriguez`.

## Build & Flash

Requires [TinyGo](https://tinygo.org/) (not standard Go) for the firmware.

```bash
# Build firmware ELF (does not flash)
make build

# Build and flash to connected Pico 2
make flash

# Adjust target/serial if needed
make flash TARGET=pico2 SERIAL=usb
```

The build uses `-ldflags` to embed `gitHash` and `buildTime` into the binary.

## Testing

**Unit tests** (standard Go, runs on host):
```bash
cd cmd/servotest && go test ./...

# Single test
cd cmd/servotest && go test -run TestParseM114_Normal
```

**Integration tests** (requires device connected via USB with servos attached):
```bash
make test-servo

# With options
cd cmd/servotest && go run . -verbose -run TestFeed -tolerance 3.0
```

The integration test tool (`cmd/servotest`) auto-detects the USB serial port (`/dev/cu.usbmodem*`) and sends real G-code commands to the device, verifying servo positions.

## Debugging

VSCode launch config uses `cortex-debug` with OpenOCD for on-device debugging. Requires a CMSIS-DAP probe and `openocd.cfg` pointing at rp2350.

## Architecture

### Build constraint

All firmware source files in the root use `//go:build rp` — they only compile under TinyGo targeting RP2xxx. The `cmd/servotest` tool is a separate Go module that runs on the host.

### Event bus pattern

Communication between components uses a publish/subscribe event bus (`pkg/event`):

- **EventBus** — central pub/sub router with topic-based channels
- **EventClient** — per-component handle for publishing/subscribing (wraps a receive channel)
- **Topics** (`pkg/topic`) — string constants for message routing: `broadcast:reply`, `broadcast:debug`, `broadcast:diag`, `rxcmd:serial`

Flow: Serial input → `rxcmd:serial` topic → CommandHandler processes → publishes response to `broadcast:reply` → Serial outputs write back to UART/USB

### Key components

- **main.go** — Axis state, G-code command handlers, coordinate system (degrees ↔ ticks). Three axes: FEED (ID 1), BEND (ID 2), ROTATE (ID 3)
- **sts3215.go** — STS3215 servo bus driver using PIO-based half-duplex UART (1Mbps on a single GPIO pin)
- **sts3215.pio** / **sts3215_pio.go** — PIO assembly for TX/RX UART. The `.go` file is auto-generated: `pioasm -o go sts3215.pio sts3215_pio.go`
- **serials.go** — Initializes serial interfaces (USB CDC + UART + PseudoSerial for stdin/stdout)
- **pkg/serial/** — Serial port abstraction with line-based and `<...>` delimited command parsing
- **pkg/command/** — Command handler registry mapping G-code commands to handler functions

### Coordinate system

Positions are in degrees. Internal conversion: `4096 ticks = 360 degrees`. Each axis tracks an `Offset` (raw tick origin) and `Position` (logical degrees). G92 recalibrates offsets without moving servos. ROTATE axis is clamped to [-180, 180].

### Servo protocol

STS3215 uses a Dynamixel-like packet protocol: `FF FF ID LEN INST PARAMS... CHECKSUM`. Communication is half-duplex on a single wire using PIO state machines for TX (open-drain) and RX. Echo bytes from TX are drained before reading servo responses.
