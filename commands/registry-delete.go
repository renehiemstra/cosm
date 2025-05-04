package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// RegistryDelete deletes a registry from the local system
func RegistryDelete(cmd *cobra.Command, args []string) error {
	registryName, err := validateStatusArgs(args) // Reusing validateStatusArgs for single argument check
	if err != nil {
		return err
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return err
	}
	registriesDir := setupRegistriesDir(cosmDir)

	// Check if registry exists
	if err := assertRegistryExists(registriesDir, registryName); err != nil {
		return err
	}

	registryPath := filepath.Join(registriesDir, registryName)
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		return fmt.Errorf("registry directory '%s' not found", registryName)
	}

	// Check for --force flag
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Printf("Are you sure you want to delete registry '%s'? [y/N]: ", registryName)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if response != "y" && response != "yes" {
			fmt.Println("Registry deletion cancelled.")
			return nil
		}
	}

	// Remove registry directory
	if err := os.RemoveAll(registryPath); err != nil {
		return fmt.Errorf("failed to delete registry directory '%s': %v", registryPath, err)
	}

	// Update registries.json
	registryNames, err := loadRegistryNames(cosmDir)
	if err != nil {
		return err
	}
	var updatedNames []string
	for _, name := range registryNames {
		if name != registryName {
			updatedNames = append(updatedNames, name)
		}
	}
	data, err := json.MarshalIndent(updatedNames, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registries.json: %v", err)
	}
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if err := os.WriteFile(registriesFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registries.json: %v", err)
	}

	fmt.Printf("Deleted registry '%s'\n", registryName)
	return nil
}
