# Issue: Multiple USB Ports / Auto-Detection

## Problem

After flashing, the Pico2 re-enumerates on a different USB port (e.g., was `/dev/cu.usbmodem1108`, now `/dev/cu.usbmodem1301`). With multiple USB serial devices connected, the test tool fails with:

```
multiple USB serial ports found: /dev/cu.usbmodem1103, /dev/cu.usbmodem1108, /dev/cu.usbmodem1301 — use -port to specify
```

This requires manually identifying the correct port each time.

## Root Cause

USB CDC devices on RP2040/RP2350 re-enumerate after flashing, often getting a new port number. macOS assigns port names based on USB topology (hub, port number), and these can change. Multiple Pico boards or other USB serial devices exacerbate the problem.

## Fix Applied

### Auto-detection by probing (`cmd/servotest/serial.go`)

When multiple `/dev/cu.usbmodem*` ports are found, the test tool now **probes each port** to find the Wirebender device:

1. Open port at 115200 baud, 8N1
2. Flush stale data (50ms timeout drain)
3. Send `help\r\n`
4. Read response with 1-second timeout
5. Check for `"Available commands"` in response — this string is unique to Wirebender's `handleHelp` output
6. Return the first matching port

```go
func detectPort() (string, error) {
    matches, _ := filepath.Glob("/dev/cu.usbmodem*")
    switch len(matches) {
    case 0:
        return "", fmt.Errorf("no USB serial ports found")
    case 1:
        return matches[0], nil
    default:
        // Probe each port for Wirebender firmware
        for _, port := range matches {
            if probePort(port) {
                return port, nil
            }
        }
        return "", fmt.Errorf("no Wirebender device found on any port")
    }
}
```

### Identification string

The firmware responds to `help` with:

```
Wirebender - Hash: <gitHash> Built: <buildTime>
Available commands:
  G0/G1 F<deg> B<deg> R<deg> S<speed>   - Move servos ...
  ...
```

The string `"Available commands"` is used as the identification marker. It's unique to the Wirebender firmware and won't match other USB serial devices.

## Behavior

| Scenario | Old behavior | New behavior |
|----------|-------------|-------------|
| 1 port | Auto-select | Auto-select (unchanged) |
| Multiple ports, 1 Wirebender | Error, manual `-port` | Auto-detect correct port |
| Multiple ports, 0 Wirebender | Error | Error with helpful message |
| `-port` flag specified | Use specified | Use specified (unchanged) |

## Remaining considerations

- If **multiple Wirebender devices** are connected, the first one found is used. A `-index` flag could be added if needed.
- Probing takes ~1 second per non-Wirebender port (timeout). This is acceptable at startup.
- The probe opens and closes the port before the main `Open()` call — no state leaks.
