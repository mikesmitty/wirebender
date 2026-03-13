# Material Calibration

This guide explains how to calibrate springback settings for new wire materials and provides a reference table of starting values for Phosphor Bronze wire.

## Introduction to Springback

When wire is bent, it has a natural tendency to spring back towards its original shape. To achieve an accurate bend, the machine must over-bend the wire to compensate for this effect.

In `dxf2bend`, springback is modeled with a linear equation:
`commanded_angle = (desired_angle * springback_multiplier) + springback_offset`

*   **Springback Multiplier (`sm`):** Accounts for the proportional springback of the material. Highly elastic materials need higher multipliers (e.g., 1.15 to 1.20).
*   **Springback Offset (`so`):** The "elastic limit" of the wire in degrees. This is the angle the machine must push the wire before any permanent plastic deformation begins.

## How to Calibrate

To perfectly dial in your settings for a specific wire:

1.  **Test Two Bends:** Bend a 90° angle and a 45° angle using your current settings.
2.  **Adjust the Offset:** If both bends are too shallow by the *same number of degrees* (e.g., the 90° bends to 85° and the 45° bends to 40°), increase your `springback_offset` by 5°.
3.  **Adjust the Multiplier:** If the error is proportional (e.g., the 90° is off by 10°, but the 45° is only off by 5°), increase your `springback_multiplier`.
4.  **Iterate:** Repeat the process until both 90° and 45° bends are accurate.

## Phosphor Bronze Starting Points

Phosphor bronze is a highly elastic and springy material. The values below provide a good, conservative starting point for calibration.

| Wire Diameter (mm) | Min Bend Radius (mm) | Springback Multiplier | Springback Offset (°) |
|--------------------|----------------------|-----------------------|-----------------------|
| 0.20               | 0.40                 | 1.15                  | 8.0                   |
| 0.25               | 0.50                 | 1.15                  | 8.0                   |
| 0.30               | 0.60                 | 1.15                  | 8.0                   |
| 0.40               | 0.80                 | 1.15                  | 8.0                   |
| 0.50               | 1.00                 | 1.15                  | 8.0                   |
| 0.60               | 1.20                 | 1.15                  | 8.0                   |
| 0.70               | 1.40                 | 1.15                  | 8.0                   |
| 0.75               | 1.50                 | 1.15                  | 8.0                   |
| 0.80               | 1.60                 | 1.15                  | 8.0                   |
| 0.90               | 1.80                 | 1.15                  | 8.0                   |
| 1.00               | 2.00                 | 1.15                  | 8.0                   |

*(Note: Minimum bend radius is conservatively set to 2x wire diameter. Multiplier and offset are consistent starting points that should be calibrated per specific machine setup and wire temper.)*
