# Remaining Test Issues

## Servo Bus Timeouts
- STS3215 servos intermittently fail to respond to position queries
- M114 returns `FEED: ERROR (timeout)` instead of a position value
- Affects all axes (FEED, BEND, ROTATE) non-deterministically
- When servos do respond, position values are accurate (e.g. 19.8 for expected 20.0)
- Likely caused by BEND/ROTATE servos being unplugged during testing — revisit if issues persist with servos connected

### Possible fixes
- [ ] Retest with all servos plugged in to confirm timeouts are resolved
- [ ] Check if bus timing/delays in `sts3215.go` need tuning

## TestRelativeModeBend Failure
- `TestRelativeModeBend` fails: `BEND not found in M114 response`
- M114 returns `BEND: ERROR (timeout)` and `ROTATE: ERROR (timeout)`
- Likely caused by BEND/ROTATE servos being unplugged — retest with servos connected
- [ ] Retest with all servos plugged in to confirm timeouts are resolved

## Industry Standard Alignment & Features
- [x] Rename FEED axis from `F` to `L` (Linear) in G-code and firmware to avoid conflict with Feedrate.
- [x] Convert `L` axis from degrees to linear distance (mm).
- [x] Make feed roller diameter configurable (e.g., via `M` code or configuration variable) to support accurate degrees-to-distance conversion.