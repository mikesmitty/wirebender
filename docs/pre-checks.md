# Pre-checks & First Bends

Before you start bending wire for the first time or after changing materials, it is highly recommended to run through these pre-checks to ensure your machine is operating safely and accurately.

## 1. Hardware & Servo Verification (No Wire)
Before loading any wire or running G-code, verify the machine's hardware and communication. The project includes a dedicated `servotest` tool to help with this.

*   Run the servo test suite to ensure the servos are responding correctly to serial commands, hitting their targets, and settling within tolerance.
*   You can run this from the project root using:
    ```bash
    go run ./cmd/servotest/
    ```

## 2. Mechanical Homing & Zeroing
Ensure the machine is mechanically zeroed before powering it on or running a job.

*   The bending pin should be in its neutral position.
*   The rotation axis should be zeroed.
*   Verify nothing is blocking the path of the bender or the wire feeder.

## 3. Material Calibration (Springback)
Before running a full DXF design, you will need to calibrate the springback for your specific wire. This ensures accurate bends.

*   **Run Test Bends:** Run a 90° and a 45° test bend.
*   **Adjust Offset (`so`):** If both bends are too shallow by the *same amount* (e.g., both are 5° short), increase your `springback_offset`.
*   **Adjust Multiplier (`sm`):** If the error is proportional (e.g., the 90° is 10° short, but the 45° is 5° short), increase the `springback_multiplier`.

For detailed calibration instructions and starting points for materials like Phosphor Bronze, see the [Material Calibration Guide](calibration.md).

## 4. Dry Run / "Soft" Wire Run
Before running your first real DXF on stiff wire, perform a safety run.

*   **Dry run:** Run the generated G-code with no wire loaded at all to watch the machine's movements. Look for any unexpected full rotations or potential collisions.
*   **Soft run:** If you have soft wire (like lead-free solder or thin copper), run the job with that first. It will bend easily without stressing the machine if the settings are wildly off, allowing you to verify the feed lengths and bend directions.

## 5. Spool & Feed Check
Improper spooling can cause feeding issues or tangles.

*   Ensure your wire spool has some light tension or drag on it. If the feeder pulls wire and then stops abruptly, the spool's momentum can cause it to unspool and tangle if it spins too freely.
*   Check that the wire is feeding straight into the machine without binding.
