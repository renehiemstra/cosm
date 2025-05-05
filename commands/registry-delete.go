package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// deleteRegistryConfig holds configuration for deleting a registry
type deleteRegistryConfig struct {
	registryName  string
	cosmDir       string
	registriesDir string
	registryPath  string
	force         bool
	registryNames []string
}

// RegistryDelete deletes a registry from the local system
func RegistryDelete(cmd *cobra.Command, args []string) error {
	// Parse arguments and initialize config
	config, err := parseDeleteArgs(cmd, args)
	if err != nil {
		return err
	}

	// Load existing registry names
	config.registryNames, err = loadRegistryNames(config.cosmDir)
	if err != nil {
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no registries available") {
			return fmt.Errorf("failed to load registry names: %v", err)
		}
		config.registryNames = []string{} // Initialize empty list if registries.json is missing or empty
	}

	// Validate registry for deletion
	if err := validateRegistryForDeletion(config); err != nil {
		return err
	}

	// Prompt for confirmation if not forced
	if err := promptForDeletion(config); err != nil {
		return err
	}

	// Delete registry and update registries.json
	if err := deleteRegistry(config); err != nil {
		return err
	}

	fmt.Printf("Deleted registry '%s'\n", config.registryName)
	return nil
}

// parseDeleteArgs parses and validates the registry name argument
func parseDeleteArgs(cmd *cobra.Command, args []string) (*deleteRegistryConfig, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("exactly one argument required (e.g., cosm registry delete <registryName>)")
	}
	registryName := args[0]
	if registryName == "" {
		return nil, fmt.Errorf("registry name cannot be empty")
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get cosm directory: %v", err)
	}
	registriesDir, err := getRegistriesDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get registries directory: %v", err)
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return nil, fmt.Errorf("failed to get force flag: %v", err)
	}

	return &deleteRegistryConfig{
		registryName:  registryName,
		cosmDir:       cosmDir,
		registriesDir: registriesDir,
		registryPath:  filepath.Join(registriesDir, registryName),
		force:         force,
	}, nil
}

// validateRegistryForDeletion checks if the registry exists and is valid
func validateRegistryForDeletion(config *deleteRegistryConfig) error {
	if err := assertRegistryExists(config.registriesDir, config.registryName); err != nil {
		return err
	}
	if _, err := os.Stat(config.registryPath); os.IsNotExist(err) {
		return fmt.Errorf("registry directory '%s' not found", config.registryPath)
	}
	return nil
}

// promptForDeletion prompts the user for confirmation if not forced
func promptForDeletion(config *deleteRegistryConfig) error {
	if !config.force {
		fmt.Printf("Are you sure you want to delete registry '%s'? [y/N]: ", config.registryName)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if response != "y" && response != "yes" {
			fmt.Println("Registry deletion cancelled.")
			return nil
		}
	}
	return nil
}

// deleteRegistry removes the registry directory and updates registries.json
func deleteRegistry(config *deleteRegistryConfig) error {
	if err := os.RemoveAll(config.registryPath); err != nil {
		return fmt.Errorf("failed to remove directory '%s': %v", config.registryPath, err)
	}

	var updatedNames []string
	for _, name := range config.registryNames {
		if name != config.registryName {
			updatedNames = append(updatedNames, name)
		}
	}
	if err := saveRegistryNames(updatedNames, config.registriesDir); err != nil {
		return err
	}
	return nil
}
