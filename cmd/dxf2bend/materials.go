package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Material holds physical properties for a wire material.
type Material struct {
	Description          string  `yaml:"description,omitempty"`
	WireDiameter         float64 `yaml:"wire_diameter"`
	SpringbackMultiplier float64 `yaml:"springback_multiplier"`
	SpringbackOffset     float64 `yaml:"springback_offset"`
	MinBendRadius        float64 `yaml:"min_bend_radius,omitempty"`
}

// MaterialLibrary is a named collection of materials.
type MaterialLibrary struct {
	Materials map[string]*Material `yaml:"materials"`
}

// defaultMaterialFile returns ~/.config/dxf2bend/materials.yaml.
func defaultMaterialFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "dxf2bend", "materials.yaml")
}

// loadMaterialLibrary reads a YAML material library from path.
// Returns an empty library (not an error) if the file doesn't exist.
func loadMaterialLibrary(path string) (*MaterialLibrary, error) {
	lib := &MaterialLibrary{Materials: make(map[string]*Material)}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lib, nil
		}
		return nil, fmt.Errorf("reading material file: %w", err)
	}

	if err := yaml.Unmarshal(data, lib); err != nil {
		return nil, fmt.Errorf("parsing material file: %w", err)
	}

	if lib.Materials == nil {
		lib.Materials = make(map[string]*Material)
	}

	return lib, nil
}

// saveMaterialLibrary writes the library to path, creating directories as needed.
func saveMaterialLibrary(path string, lib *MaterialLibrary) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err := enc.Encode(lib)
	enc.Close()
	if err != nil {
		return fmt.Errorf("marshaling materials: %w", err)
	}
	data := buf.Bytes()

	header := []byte("# dxf2bend material properties library\n# Springback values are machine-specific — calibrate on your setup.\n#\n# Usage:\n#   dxf2bend shape.dxf --material <name>\n#   dxf2bend material list\n#   dxf2bend material save myWire --wire 1.0 --sm 1.15 --so 2.0\n\n")
	return os.WriteFile(path, append(header, data...), 0644)
}

// listMaterials prints a formatted table of all materials to stderr.
func listMaterials(lib *MaterialLibrary) {
	if len(lib.Materials) == 0 {
		fmt.Fprintln(os.Stderr, "No materials defined.")
		fmt.Fprintf(os.Stderr, "\nSave one with: dxf2bend material save <name> --wire <dia> --sm <mult> --so <offset>\n")
		return
	}

	names := make([]string, 0, len(lib.Materials))
	for name := range lib.Materials {
		names = append(names, name)
	}
	sort.Strings(names)

	w := tabwriter.NewWriter(os.Stderr, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tWIRE DIA\tSM\tSO\tMIN RADIUS\tDESCRIPTION")
	fmt.Fprintln(w, "----\t--------\t--\t--\t----------\t-----------")
	for _, name := range names {
		m := lib.Materials[name]
		minR := "-"
		if m.MinBendRadius > 0 {
			minR = fmt.Sprintf("%.2f", m.MinBendRadius)
		}
		fmt.Fprintf(w, "%s\t%.2f mm\t%.3f\t%.2f°\t%s\t%s\n",
			name, m.WireDiameter, m.SpringbackMultiplier, m.SpringbackOffset, minR, m.Description)
	}
	w.Flush()
}
