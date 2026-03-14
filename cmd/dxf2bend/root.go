package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/spf13/cobra"
	"github.com/yofu/dxf"
)

var version = "dev"

var (
	// Persistent flags (available to all subcommands)
	materialFile  string
	verbose       bool
	previewTarget string
	previewTheme  string

	// Root command flags
	outputPath    string
	feedScale     float64
	speed         int
	speedStraight int
	speedBend     int
	springbackM   float64
	springbackO   float64
	mandrelRadius float64
	wireDia       float64
	materialName  string
	arcResolution float64
	pathID        string
	pathIndex     int
	combinePaths  bool
	simplifyTol   float64
	reversePath   bool
	strict        bool
)

var rootCmd = &cobra.Command{
	Use:     "dxf2bend [input.dxf]",
	Short:   "Convert DXF files to Wirebender G-code",
	Version: version,
	Long: `dxf2bend reads a DXF file containing lines, arcs, splines, and polylines,
and generates G-code for the Wirebender wire bending machine.

Configuration is loaded from ~/.config/dxf2bend/config.yaml if present.
CLI flags always override config file values.`,
	Args:          cobra.MaximumNArgs(1),
	RunE:          runConvert,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Load config and env vars before any command runs
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		snapshotCLIFlags()
		cfg, err := loadConfig(defaultConfigFile())
		if err != nil {
			return err
		}
		applyDefaults(cfg)
		return nil
	}

	// Persistent flags — available on all subcommands
	rootCmd.PersistentFlags().StringVar(&materialFile, "material-file", "",
		"Path to materials YAML file (default ~/.config/dxf2bend/materials.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().StringVar(&previewTarget, "preview", "terminal", "Preview destination ('terminal', 'none', or a file path like 'out.png')")
	rootCmd.PersistentFlags().StringVar(&previewTheme, "preview-theme", "cyberpunk", "Preview color theme ('vibrant' or 'cyberpunk')")
	rootCmd.PersistentFlags().StringVar(&pathID, "path-id", "", "ID of path to use")
	rootCmd.PersistentFlags().IntVar(&pathIndex, "path-index", -1, "Index of path to use (-1 = first path)")
	rootCmd.PersistentFlags().BoolVar(&combinePaths, "combine-paths", false, "Concatenate all paths into one")

	// Root-only flags
	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output G-code file (default stdout)")
	rootCmd.Flags().Float64Var(&feedScale, "scale", 1.0, "Scale factor for feed distance")
	rootCmd.Flags().IntVar(&speed, "speed", 500, "Servo speed")
	rootCmd.Flags().IntVar(&speedStraight, "speed-straight", 0, "Servo speed for feed-only moves (0 = use --speed)")
	rootCmd.Flags().IntVar(&speedBend, "speed-bend", 0, "Servo speed for bend moves (0 = use --speed)")
	rootCmd.Flags().Float64Var(&springbackM, "sm", 1.0, "Springback multiplier (commanded = desired * sm + so)")
	rootCmd.Flags().Float64Var(&springbackO, "so", 0.0, "Springback offset in degrees")
	rootCmd.Flags().Float64Var(&mandrelRadius, "mandrel", 0.0, "Mandrel radius in mm")
	rootCmd.Flags().Float64Var(&wireDia, "wire", 0.0, "Wire diameter in mm")
	rootCmd.Flags().StringVar(&materialName, "material", "", "Material name from library")
	rootCmd.Flags().Float64Var(&arcResolution, "arc-resolution", 5.0, "Arc discretization step in degrees")
	rootCmd.Flags().Float64Var(&simplifyTol, "simplify", 0.0, "Ramer-Douglas-Peucker simplification tolerance in mm (0 = disabled)")
	rootCmd.Flags().BoolVar(&reversePath, "reverse", false, "Reverse the point order before generating G-code")
	rootCmd.Flags().BoolVar(&strict, "strict", false, "Exit with error when segments are too short for bend radius")
}

func resolveMaterialFile() string {
	if materialFile != "" {
		return materialFile
	}
	return defaultMaterialFile()
}

func runConvert(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	inputPath := args[0]

	// Load material properties (CLI flags override material values)
	if materialName != "" {
		lib, err := loadMaterialLibrary(resolveMaterialFile())
		if err != nil {
			return fmt.Errorf("failed to load material library: %w", err)
		}
		mat, ok := lib.Materials[materialName]
		if !ok {
			fmt.Fprintf(os.Stderr, "Unknown material %q. Available materials:\n\n", materialName)
			listMaterials(lib)
			os.Exit(1)
		}

		// Only apply material value if the user didn't explicitly set the flag
		if !cmd.Flags().Changed("wire") {
			wireDia = mat.WireDiameter
		}
		if !cmd.Flags().Changed("sm") {
			springbackM = mat.SpringbackMultiplier
		}
		if !cmd.Flags().Changed("so") {
			springbackO = mat.SpringbackOffset
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "Using material %q: wire=%.2fmm sm=%.3f so=%.2f°\n",
				materialName, wireDia, springbackM, springbackO)
		}

		// Warn if bend radius is below material minimum
		totalR := mandrelRadius + (wireDia / 2.0)
		if mat.MinBendRadius > 0 && totalR > 0 && totalR < mat.MinBendRadius {
			fmt.Fprintf(os.Stderr, "WARNING: Total bend radius (%.2f mm) is below material minimum (%.2f mm)\n",
				totalR, mat.MinBendRadius)
		}
	}

	// Resolve per-move speeds
	effSpeedStraight := speed
	if speedStraight != 0 {
		effSpeedStraight = speedStraight
	}
	effSpeedBend := speed
	if speedBend != 0 {
		effSpeedBend = speedBend
	}

	wp, err := parseDXFToWirePath(inputPath)
	if err != nil {
		return err
	}

	if previewTarget != "none" {
		if err := generatePreview(wp, previewTarget); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: failed to generate preview: %v\n", err)
		}
	}

	gcode, err := generateGCodeFromPath(wp, feedScale, effSpeedStraight, effSpeedBend, springbackM, springbackO, verbose)
	if err != nil {
		return err
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(gcode), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	} else {
		fmt.Print(gcode)
	}

	return nil
}

func parseDXFPaths(inputPath string) ([][]mgl64.Vec3, []string, error) {
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read DXF: %w", err)
	}

	// Strip all Linetype references (Group Code 6) to avoid missing linetype errors
	re := regexp.MustCompile("(?m)^[ \\t]*6\\r?\\n.*\\r?\\n")
	cleaned := re.ReplaceAllString(string(b), "")

	d, err := dxf.FromReader(strings.NewReader(cleaned))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load DXF: %w", err)
	}

	paths := extractPaths(d, arcResolution, verbose)
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("no paths found in DXF")
	}

	var ids []string
	for _, p := range paths {
		ids = append(ids, hashPath(p))
	}

	return paths, ids, nil
}

func parseDXFToWirePath(inputPath string) (*WirePath, error) {
	paths, ids, err := parseDXFPaths(inputPath)
	if err != nil {
		return nil, err
	}

	// Report all paths
	if len(paths) > 1 {
		fmt.Fprintf(os.Stderr, "WARNING: Found %d separate paths in DXF:\n", len(paths))
		for i, p := range paths {
			fmt.Fprintf(os.Stderr, "  Path %d (ID: %s): %d points\n", i, ids[i], len(p))
		}
	}

	// Select or combine paths
	var points []mgl64.Vec3
	if combinePaths {
		if verbose {
			fmt.Fprintf(os.Stderr, "Combining all %d paths\n", len(paths))
		}
		for _, p := range paths {
			points = append(points, p...)
		}
	} else if pathID != "" {
		found := false
		for i, id := range ids {
			if id == pathID {
				points = paths[i]
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("path ID '%s' not found", pathID)
		}
	} else {
		if len(paths) > 1 && pathIndex < 0 {
			fmt.Fprintf(os.Stderr, "ERROR: Found %d separate paths in DXF. Please select one:\n", len(paths))
			for i, p := range paths {
				fmt.Fprintf(os.Stderr, "  Path %d (ID: %s): %d points\n", i, ids[i], len(p))
			}
			fmt.Fprintf(os.Stderr, "\nUse --path-id <id>, --path-index <index>, or --combine-paths to select.\n")
			fmt.Fprintf(os.Stderr, "Use 'dxf2bend preview %s' to view them graphically.\n", inputPath)
			return nil, fmt.Errorf("multiple paths found")
		}
		idx := 0
		if pathIndex >= 0 {
			idx = pathIndex
		}
		if idx >= len(paths) {
			return nil, fmt.Errorf("path index %d out of range (have %d paths)", idx, len(paths))
		}
		points = paths[idx]
	}

	if len(points) < 2 {
		return nil, fmt.Errorf("selected path too short (need at least 2 points)")
	}

	// Path simplification
	if simplifyTol > 0 {
		before := len(points)
		points = rdpSimplify(points, simplifyTol)
		if verbose {
			fmt.Fprintf(os.Stderr, "Simplified path: %d -> %d points (tolerance %.3f mm)\n", before, len(points), simplifyTol)
		}
	}

	// Reverse path
	if reversePath {
		if verbose {
			fmt.Fprintf(os.Stderr, "Reversing path (%d points)\n", len(points))
		}
		for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
			points[i], points[j] = points[j], points[i]
		}
	}

	totalRadius := mandrelRadius + (wireDia / 2.0)
	wp, err := calculateWirePath(points, springbackM, springbackO, totalRadius, materialName, strict, verbose)
	if err != nil {
		return nil, err
	}
	return wp, nil
}
