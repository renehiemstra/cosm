package commands

import (
	"bufio"
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// RegistryClone clones a registry from a Git URL
func RegistryClone(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one argument required (e.g., cosm registry clone <giturl>)")
	}
	gitURL := args[0]
	if gitURL == "" {
		return fmt.Errorf("git URL cannot be empty")
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return err
	}
	registriesDir := setupRegistriesDir(cosmDir)

	// Clone to a temporary directory to read registry.json
	tmpDir, err := os.MkdirTemp("", "cosm-clone-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := exec.Command("git", "clone", gitURL, tmpDir).Run(); err != nil {
		return fmt.Errorf("failed to clone repository at '%s': %v", gitURL, err)
	}

	// Read registry.json to get registry name
	registryMetaFile := filepath.Join(tmpDir, "registry.json")
	data, err := os.ReadFile(registryMetaFile)
	if err != nil {
		return fmt.Errorf("failed to read registry.json from cloned repository: %v", err)
	}
	var registry types.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return fmt.Errorf("failed to parse registry.json: %v", err)
	}
	if registry.Name == "" {
		return fmt.Errorf("registry.json does not contain a valid registry name")
	}

	// Check for existing registry
	registryNames, err := loadRegistryNames(cosmDir)
	if err != nil {
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no registries available") {
			return err
		}
		registryNames = []string{} // Initialize empty list if registries.json is missing or empty
	}
	registryPath := filepath.Join(registriesDir, registry.Name)
	if contains(registryNames, registry.Name) {
		fmt.Printf("Registry '%s' already exists. Overwrite? [y/N]: ", registry.Name)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if response != "y" && response != "yes" {
			fmt.Println("Registry clone cancelled.")
			return nil
		}
		// Delete existing registry (without --force to align with prompt)
		deleteCmd := &cobra.Command{}
		if err := RegistryDelete(deleteCmd, []string{registry.Name}); err != nil {
			return fmt.Errorf("failed to delete existing registry '%s': %v", registry.Name, err)
		}
	}

	// Clone to final location
	if err := exec.Command("git", "clone", gitURL, registryPath).Run(); err != nil {
		return fmt.Errorf("failed to clone repository to '%s': %v", registryPath, err)
	}

	// Update registries.json
	registryNames = append(registryNames, registry.Name)
	data, err = json.MarshalIndent(registryNames, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registries.json: %v", err)
	}
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if err := os.WriteFile(registriesFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registries.json: %v", err)
	}

	fmt.Printf("Cloned registry '%s' from %s\n", registry.Name, gitURL)
	return nil
}
