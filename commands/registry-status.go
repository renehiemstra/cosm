package commands

import (
	"cosm/types"
	"fmt"

	"github.com/spf13/cobra"
)

// statusRegistryConfig holds configuration for displaying registry status
type statusRegistryConfig struct {
	registryName  string
	cosmDir       string
	registriesDir string
	registry      types.Registry
	registryFile  string
}

// RegistryStatus prints an overview of packages in a registry
func RegistryStatus(cmd *cobra.Command, args []string) error {
	// Parse arguments and initialize config
	config, err := parseStatusArgs(args)
	if err != nil {
		return err
	}

	// Validate registry and load metadata
	if err := validateRegistryForStatus(config); err != nil {
		return err
	}

	// Print registry status
	printRegistryStatus(config)
	return nil
}

// parseStatusArgs parses and validates the registry name
func parseStatusArgs(args []string) (*statusRegistryConfig, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("exactly one argument required (e.g., cosm registry status <registryName>)")
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

	return &statusRegistryConfig{
		registryName:  registryName,
		cosmDir:       cosmDir,
		registriesDir: registriesDir,
	}, nil
}

// validateRegistryForStatus checks if the registry exists and loads its metadata
func validateRegistryForStatus(config *statusRegistryConfig) error {
	if err := assertRegistryExists(config.registriesDir, config.registryName); err != nil {
		return fmt.Errorf("failed to validate registry '%s': %v", config.registryName, err)
	}

	var err error
	config.registry, config.registryFile, err = LoadRegistryMetadata(config.registriesDir, config.registryName)
	if err != nil {
		return fmt.Errorf("failed to load registry metadata for '%s': %v", config.registryName, err)
	}
	return nil
}

// printRegistryStatus displays the registry's package information
func printRegistryStatus(config *statusRegistryConfig) {
	fmt.Printf("Registry Status for '%s':\n", config.registryName)
	if len(config.registry.Packages) == 0 {
		fmt.Println("  No packages registered.")
	} else {
		fmt.Println("  Packages:")
		for pkgName, pkgInfo := range config.registry.Packages {
			fmt.Printf("    - %s (UUID: %s)\n", pkgName, pkgInfo.UUID)
		}
	}
}
