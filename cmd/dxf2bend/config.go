package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// Config holds persistent default values for dxf2bend flags.
type Config struct {
	Mandrel       float64 `yaml:"mandrel,omitempty"`
	Speed         int     `yaml:"speed,omitempty"`
	SpeedStraight int     `yaml:"speed_straight,omitempty"`
	SpeedBend     int     `yaml:"speed_bend,omitempty"`
	Scale         float64 `yaml:"scale,omitempty"`
	ArcResolution float64 `yaml:"arc_resolution,omitempty"`
	Material      string  `yaml:"material,omitempty"`
	MaterialFile  string  `yaml:"material_file,omitempty"`
	Strict        bool    `yaml:"strict,omitempty"`
}

// envBindings maps flag names to environment variable names.
var envBindings = []struct {
	flag string
	env  string
}{
	{"mandrel", "DXF2BEND_MANDREL"},
	{"speed", "DXF2BEND_SPEED"},
	{"speed-straight", "DXF2BEND_SPEED_STRAIGHT"},
	{"speed-bend", "DXF2BEND_SPEED_BEND"},
	{"scale", "DXF2BEND_SCALE"},
	{"arc-resolution", "DXF2BEND_ARC_RESOLUTION"},
	{"material", "DXF2BEND_MATERIAL"},
	{"strict", "DXF2BEND_STRICT"},
	{"sm", "DXF2BEND_SM"},
	{"so", "DXF2BEND_SO"},
	{"wire", "DXF2BEND_WIRE"},
}

const envMaterialFile = "DXF2BEND_MATERIAL_FILE"

// defaultConfigFile returns ~/.config/dxf2bend/config.yaml.
func defaultConfigFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "dxf2bend", "config.yaml")
}

// loadConfig reads the config file. Returns a zero Config if the file doesn't exist.
func loadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config file: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config file: %w", err)
	}
	return cfg, nil
}

// applyDefaults loads config file values and then environment variables onto
// flags that weren't explicitly set on the CLI.
// Precedence: CLI flags > env vars > config file > flag defaults.
func applyDefaults(cfg Config) {
	// First pass: apply config file values (lowest priority of the two)
	applyConfigToFlags(cfg)
	// Second pass: env vars override config (but not CLI flags)
	applyEnvToFlags()
}

// applyConfigToFlags sets flag values from the config file for flags not set on the CLI.
func applyConfigToFlags(cfg Config) {
	setIfUnchanged := func(name string, val any) {
		f := rootCmd.Flags().Lookup(name)
		if f == nil || f.Changed {
			return
		}
		switch v := val.(type) {
		case float64:
			if v != 0 {
				rootCmd.Flags().Set(name, fmt.Sprintf("%g", v))
			}
		case int:
			if v != 0 {
				rootCmd.Flags().Set(name, fmt.Sprintf("%d", v))
			}
		case string:
			if v != "" {
				rootCmd.Flags().Set(name, v)
			}
		case bool:
			if v {
				rootCmd.Flags().Set(name, "true")
			}
		}
	}

	setIfUnchanged("mandrel", cfg.Mandrel)
	setIfUnchanged("speed", cfg.Speed)
	setIfUnchanged("speed-straight", cfg.SpeedStraight)
	setIfUnchanged("speed-bend", cfg.SpeedBend)
	setIfUnchanged("scale", cfg.Scale)
	setIfUnchanged("arc-resolution", cfg.ArcResolution)
	setIfUnchanged("material", cfg.Material)
	setIfUnchanged("strict", cfg.Strict)

	// material-file is a persistent flag on rootCmd
	if !rootCmd.PersistentFlags().Lookup("material-file").Changed && cfg.MaterialFile != "" {
		rootCmd.PersistentFlags().Set("material-file", cfg.MaterialFile)
	}
}

// applyEnvToFlags overrides flag values from environment variables.
// Only applies to flags not explicitly set on the CLI (config values are overridden).
func applyEnvToFlags() {
	for _, b := range envBindings {
		val := os.Getenv(b.env)
		if val == "" {
			continue
		}
		f := rootCmd.Flags().Lookup(b.flag)
		if f == nil {
			continue
		}
		// CLI flags always win — skip if user set it on the command line.
		// We need to check if it was set by the user (not by applyConfigToFlags).
		// Since config was applied first via Set(), Changed is true for both.
		// We track the original CLI-changed state before config was applied.
		if cliChangedFlags[b.flag] {
			continue
		}
		// Validate the value before setting
		switch f.Value.Type() {
		case "float64":
			if _, err := strconv.ParseFloat(val, 64); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: invalid %s=%q (expected number), ignoring\n", b.env, val)
				continue
			}
		case "int":
			if _, err := strconv.Atoi(val); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: invalid %s=%q (expected integer), ignoring\n", b.env, val)
				continue
			}
		case "bool":
			if _, err := strconv.ParseBool(val); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: invalid %s=%q (expected true/false), ignoring\n", b.env, val)
				continue
			}
		}
		rootCmd.Flags().Set(b.flag, val)
	}

	// material-file persistent flag
	if val := os.Getenv(envMaterialFile); val != "" && !cliChangedFlags["material-file"] {
		rootCmd.PersistentFlags().Set("material-file", val)
	}
}

// cliChangedFlags tracks which flags were set on the CLI (before config/env are applied).
var cliChangedFlags map[string]bool

// snapshotCLIFlags records which flags were explicitly set via CLI args.
// Must be called before applyDefaults.
func snapshotCLIFlags() {
	cliChangedFlags = make(map[string]bool)
	rootCmd.Flags().Visit(func(f *pflag.Flag) {
		cliChangedFlags[f.Name] = true
	})
	rootCmd.PersistentFlags().Visit(func(f *pflag.Flag) {
		cliChangedFlags[f.Name] = true
	})
}
