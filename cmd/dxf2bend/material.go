package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var materialCmd = &cobra.Command{
	Use:   "material",
	Short: "Manage the material properties library",
	Long:  `View, save, and delete material definitions used for springback compensation.`,
}

// --- material list ---

var materialListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved materials",
	Args:  cobra.NoArgs,
	RunE:  runMaterialList,
}

func runMaterialList(cmd *cobra.Command, args []string) error {
	path := resolveMaterialFile()
	lib, err := loadMaterialLibrary(path)
	if err != nil {
		return fmt.Errorf("failed to load material library: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Material file: %s\n\n", path)
	listMaterials(lib)
	return nil
}

// --- material show ---

var materialShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a specific material",
	Args:  cobra.ExactArgs(1),
	RunE:  runMaterialShow,
}

func runMaterialShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	lib, err := loadMaterialLibrary(resolveMaterialFile())
	if err != nil {
		return fmt.Errorf("failed to load material library: %w", err)
	}
	mat, ok := lib.Materials[name]
	if !ok {
		return fmt.Errorf("material %q not found", name)
	}
	fmt.Printf("Name:                  %s\n", name)
	if mat.Description != "" {
		fmt.Printf("Description:           %s\n", mat.Description)
	}
	fmt.Printf("Wire Diameter:         %.2f mm\n", mat.WireDiameter)
	fmt.Printf("Springback Multiplier: %.3f\n", mat.SpringbackMultiplier)
	fmt.Printf("Springback Offset:     %.2f deg\n", mat.SpringbackOffset)
	if mat.MinBendRadius > 0 {
		fmt.Printf("Min Bend Radius:       %.2f mm\n", mat.MinBendRadius)
	}
	return nil
}

// --- material save ---

var (
	saveWire      float64
	saveSM        float64
	saveSO        float64
	saveMinRadius float64
	saveDesc      string
)

var materialSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save a material to the library",
	Args:  cobra.ExactArgs(1),
	RunE:  runMaterialSave,
}

func runMaterialSave(cmd *cobra.Command, args []string) error {
	name := args[0]
	path := resolveMaterialFile()
	lib, err := loadMaterialLibrary(path)
	if err != nil {
		return fmt.Errorf("failed to load material library: %w", err)
	}
	lib.Materials[name] = &Material{
		Description:          saveDesc,
		WireDiameter:         saveWire,
		SpringbackMultiplier: saveSM,
		SpringbackOffset:     saveSO,
		MinBendRadius:        saveMinRadius,
	}
	if err := saveMaterialLibrary(path, lib); err != nil {
		return fmt.Errorf("failed to save material: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Saved material %q to %s\n", name, path)
	return nil
}

// --- material delete ---

var materialDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a material from the library",
	Args:  cobra.ExactArgs(1),
	RunE:  runMaterialDelete,
}

func runMaterialDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	path := resolveMaterialFile()
	lib, err := loadMaterialLibrary(path)
	if err != nil {
		return fmt.Errorf("failed to load material library: %w", err)
	}
	if _, ok := lib.Materials[name]; !ok {
		return fmt.Errorf("material %q not found", name)
	}
	delete(lib.Materials, name)
	if err := saveMaterialLibrary(path, lib); err != nil {
		return fmt.Errorf("failed to save material library: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Deleted material %q from %s\n", name, path)
	return nil
}

func init() {
	rootCmd.AddCommand(materialCmd)
	materialCmd.AddCommand(materialListCmd)
	materialCmd.AddCommand(materialShowCmd)
	materialCmd.AddCommand(materialSaveCmd)
	materialCmd.AddCommand(materialDeleteCmd)

	// Flags for material save
	materialSaveCmd.Flags().Float64Var(&saveWire, "wire", 0.0, "Wire diameter in mm")
	materialSaveCmd.Flags().Float64Var(&saveSM, "sm", 1.0, "Springback multiplier")
	materialSaveCmd.Flags().Float64Var(&saveSO, "so", 0.0, "Springback offset in degrees")
	materialSaveCmd.Flags().Float64Var(&saveMinRadius, "min-bend-radius", 0.0, "Minimum bend radius in mm")
	materialSaveCmd.Flags().StringVar(&saveDesc, "description", "", "Material description")
}
