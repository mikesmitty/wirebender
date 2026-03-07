# Coordinate System Testing Checklist

Flash the firmware, connect via serial, and run through these steps in order.

## 1. Help

```
help
```

Confirm output lists: G0/G1, G28, G90, G91, G92, M17, M18, M112, M114, M122, M119, M121, M120, M400, help.

## 2. Startup Mode

On boot, serial output should include:

```
Mode: Absolute (G90)
```

## 3. Move in Degrees

```
G0 L90
```

- Linear servo should physically move ~90 degrees (1024 ticks).
- Response: `ok`

## 4. Read Position in Degrees

```
M114
```

- Expected: `LINEAR: 90.0 BEND: 0.0 ROTATE: 0.0` (approximately)

## 5. Full Status Shows Degrees + Raw

```
M122
```

- FEED line should show `Pos:90.0 Raw:1024` (approximately), plus Speed/Load/Volt/Temp fields.

## 6. Zero All Axes

```
G92
```

- Response: `ok`

```
M114
```

- Expected: `LINEAR: 0.0 BEND: 0.0 ROTATE: 0.0`
- Servos should NOT move — only the coordinate offset changes.

## 7. Move From New Zero

```
G0 L45
```

- Linear feed moves 45 degrees from the zeroed position.

```
M114
```

- Expected: `LINEAR: 45.0 ...`

## 8. Relative Mode

```
G91
```

- Response: `ok`

```
G0 L10
```

- Linear feed should move an additional 10 degrees (to 55 total).

```
M114
```

- Expected: `LINEAR: 55.0 ...`

## 9. Back to Absolute Mode

```
G90
```

- Response: `ok`

```
G0 L45
```

- Linear feed should move back to 45 degrees (not 45 more).

```
M114
```

- Expected: `LINEAR: 45.0 ...`

## 10. Home

```
G28
```

- All axes return to their zero position (the position set by G92).
- Response: `ok`

```
M114
```

- Expected: `LINEAR: 0.0 BEND: 0.0 ROTATE: 0.0`

## 11. Partial Home

```
G0 L30 B60
```

```
G28 L
```

- Only linear returns to zero; bend stays at 60.

```
M114
```

- Expected: `LINEAR: 0.0 BEND: 60.0 ROTATE: 0.0`

## 12. G92 With Value

```
G0 L90
```

```
G92 L45
```

- Declares current physical position as 45 degrees (not zero).

```
M114
```

- Expected: `LINEAR: 45.0 ...`

## 13. Emergency Stop

```
M112
```

- Response: `EMERGENCY STOP`
- All servos lose torque immediately — they should go limp.
- Bus is disabled.

To recover:

```
M17
```

- This should fail or have no effect since bus is disabled.

```
M400 P26
```

- Re-initialize the bus (use whatever pin you're on).

```
M17
```

- Torque re-enabled, servos hold position.
