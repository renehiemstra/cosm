package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cosm/types"

	"github.com/spf13/cobra"
)

// RegistryClone clones a registry from a Git URL to the registries directory
func RegistryClone(cmd *cobra.Command, args []string) error {
	// Validate and parse arguments
	if len(args) != 1 {
		return fmt.Errorf("exactly one argument required (e.g., cosm registry clone <giturl>)")
	}
	gitURL := args[0]
	if gitURL == "" {
		return fmt.Errorf("git URL cannot be empty")
	}

	// Initialize paths
	cosmDir, err := getCosmDir()
	if err != nil {
		return fmt.Errorf("failed to get cosm directory: %v", err)
	}
	registriesDir := filepath.Join(cosmDir, "registries")
	if err := os.MkdirAll(registriesDir, 0755); err != nil {
		return fmt.Errorf("failed to create registries directory %s: %v", registriesDir, err)
	}

	// Step 1: Clone to temporary folder
	tmpDir := filepath.Join(registriesDir, "tmp-registry-clone")
	if err := cloneToTempRegistryDir(gitURL, registriesDir, tmpDir); err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir) // Ensure cleanup

	// Step 2: Extract registry name
	registryName, err := extractRegistryName(tmpDir)
	if err != nil {
		return err
	}

	// Step 3: Check if registry name exists
	if err := checkRegistryNameDoesNotExist(registriesDir, registryName); err != nil {
		return err
	}

	// Step 4: Move temporary folder to final location
	finalDir := filepath.Join(registriesDir, registryName)
	if err := moveTempToFinalRegistryDir(tmpDir, finalDir); err != nil {
		return err
	}

	// Step 5: Add registry name to registries.json
	if err := addRegistryNameToJSON(registriesDir, registryName); err != nil {
		return err
	}

	// Step 6: Cleanup handled by defer
	fmt.Printf("Cloned registry '%s' from %s\n", registryName, gitURL)
	return nil
}

// cloneToTempRegistryDir clones the repository to a temporary directory
func cloneToTempRegistryDir(gitURL, registriesDir, tmpDir string) error {
	if err := os.RemoveAll(tmpDir); err != nil {
		return fmt.Errorf("failed to remove existing temporary directory %s: %v", tmpDir, err)
	}
	if _, err := clone(gitURL, registriesDir, "tmp-registry-clone"); err != nil {
		return fmt.Errorf("failed to clone repository from '%s' to %s: %v", gitURL, tmpDir, err)
	}
	return nil
}

// extractRegistryName reads registry.json and extracts the registry name
func extractRegistryName(tmpDir string) (string, error) {
	registryMetaFile := filepath.Join(tmpDir, "registry.json")
	data, err := os.ReadFile(registryMetaFile)
	if err != nil {
		return "", fmt.Errorf("failed to read %s from cloned repository: %v", registryMetaFile, err)
	}
	var registry types.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return "", fmt.Errorf("failed to parse %s: %v", registryMetaFile, err)
	}
	if registry.Name == "" {
		return "", fmt.Errorf("%s does not contain a valid registry name", registryMetaFile)
	}
	return registry.Name, nil
}

// checkRegistryNameDoesNotExist checks if the registry name exists in registries.json
func checkRegistryNameDoesNotExist(registriesDir, registryName string) error {
	registryNames, err := loadRegistryNames(registriesDir)
	if err != nil {
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no registries available") {
			return fmt.Errorf("failed to load registry names: %v", err)
		}
		registryNames = []string{}
	}
	for _, name := range registryNames {
		if name == registryName {
			return fmt.Errorf("registry '%s' already exists in registries.json", registryName)
		}
	}
	return nil
}

// moveTempToFinalRegistryDir moves the temporary directory to the final registry location
func moveTempToFinalRegistryDir(tmpDir, finalDir string) error {
	if err := os.MkdirAll(filepath.Dir(finalDir), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %v", finalDir, err)
	}
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return fmt.Errorf("failed to move %s to %s: %v", tmpDir, finalDir, err)
	}
	return nil
}

// addRegistryNameToJSON adds the registry name to registries.json
func addRegistryNameToJSON(registriesDir, registryName string) error {
	registryNames, err := loadRegistryNames(registriesDir)
	if err != nil {
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no registries available") {
			return fmt.Errorf("failed to load registry names: %v", err)
		}
		registryNames = []string{}
	}
	registryNames = append(registryNames, registryName)
	if err := saveRegistryNames(registryNames, registriesDir); err != nil {
		return fmt.Errorf("failed to update registries.json: %v", err)
	}
	return nil
}
