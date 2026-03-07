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

## Firmware — Critical

### Motion Command Error Feedback
- [ ] `SetPosition()` is fire-and-forget — returns "ok" even if servos are disconnected
- [ ] Verify servo accepted command; return error if axis is unreachable

### Servo Health Monitoring
- [ ] Add periodic monitoring task (every 5-10s) that reads temp, voltage, and load
- [ ] Define threshold constants (max temp, min voltage, max load) and warn/halt when exceeded

### Position Limits
- [ ] Add configurable per-axis min/max bounds (soft limits)
- [ ] Reject motion commands that exceed limits
- [ ] Optionally verify actual servo position after moves

### Bus Disconnection Handling
- [ ] Add periodic servo heartbeat (ping all servos every 10-20s)
- [ ] Maintain per-servo online/offline status flag
- [ ] Reject motion commands targeting offline servos

## Firmware — Important

### Motion Completion Semantics
- [ ] Add a "wait for idle" command (e.g., M400) that blocks until all motion is complete
- [ ] Optionally report "moving" vs "idle" status in M114/M122 responses

### Speed Limit Validation
- [ ] Define a `MaxSpeed` constant based on servo specs
- [ ] Clamp speed in `handleMotion()` and `handleHome()`; warn if clamped

### Startup Verification
- [ ] Require at least one successful servo ping before proceeding
- [ ] If all servos unreachable, print warning and enter diagnostic-only mode

### Command Parsing Errors
- [ ] Report `ParseFloat` errors on malformed parameters (e.g., `L45.5.5`) instead of silently ignoring
- [ ] Support G-code comment syntax (`; comment`)

### Coordinate Persistence
- [ ] Save offsets and relative mode to flash (M500 Save / M501 Load)
- [ ] Restore on startup with CRC integrity check

### G2/G3 Arc Support
- [ ] Implement G2 (clockwise arc) and G3 (counter-clockwise arc) commands in firmware
- [ ] Discretize arcs into linear segments for servo execution

## dxf2bend — High Priority

### Arc/Circle Entity Support
- [ ] Add DXF Arc entity support with configurable discretization resolution (e.g., `-arc-resolution 5`)
- [ ] Handle Circle entities (error or convert to full arc)

### Multi-Path Selection
- [ ] Add `-path-index N` flag to select which path to convert
- [ ] Add `-combine-paths` flag to merge paths sequentially

### Spline Support
- [ ] Add Bezier/B-spline entity support with configurable tolerance (`-spline-tolerance`)

### Path Simplification
- [ ] Implement Ramer-Douglas-Peucker simplification (`-simplify` flag) to reduce excessive G-code from dense point clouds

## dxf2bend — Medium Priority

### Bend Angle Warnings
- [ ] Warn if any bend angle exceeds machine limits (ROTATE ±180°)

### Speed Optimization
- [ ] Add `-speed-straight` and `-speed-bend` flags for different motion speeds

### Reverse Path Option
- [ ] Add `-reverse` flag to flip path direction

### Tangent Distance Validation
- [ ] Warn clearly when segments are too short for the configured bend radius instead of silently clamping