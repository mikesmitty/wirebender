# Remaining Test Issues

## Servo Bus Timeouts
- STS3215 servos intermittently fail to respond to position queries
- M114 returns `FEED: ERROR (timeout)` instead of a position value
- Affects all axes (FEED, BEND, ROTATE) non-deterministically
- When servos do respond, position values are accurate (e.g. 19.8 for expected 20.0)
- Likely hardware-level: loose wiring, bus speed, or power issues

### Possible fixes
- [ ] Investigate physical bus connections and wiring
- [ ] Add retry logic to `AssertPosition` in `cmd/servotest/tests.go` to retry M114 on transient timeout
- [ ] Consider adding retry logic to `handleGetPosition` in firmware (`main.go:354`) for servo reads
- [ ] Check if bus timing/delays in `sts3215.go` need tuning

## Multiple USB Ports
- After flashing, Pico2 re-enumerates on a different port (was 1108, now 1301)
- Three ports present: `/dev/cu.usbmodem1103`, `/dev/cu.usbmodem1108`, `/dev/cu.usbmodem1301`
- Port 1108 still shows old firmware (separate device?)
- [ ] Determine which devices are on which ports
- [ ] Consider auto-detecting the correct port by sending `help` and checking for `Available commands`
