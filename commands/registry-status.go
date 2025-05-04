package commands

import (
	"cosm/types"
	"fmt"

	"github.com/spf13/cobra"
)

// RegistryStatus prints an overview of packages in a registry
func RegistryStatus(cmd *cobra.Command, args []string) error {
	registryName, err := validateStatusArgs(args) // Updated to handle error
	if err != nil {
		return err
	}
	cosmDir, err := getCosmDir() // Already returns error
	if err != nil {
		return err
	}
	registriesDir := setupRegistriesDir(cosmDir)
	if err := assertRegistryExists(registriesDir, registryName); err != nil { // Updated to handle error
		return err
	}
	registry, _, err := LoadRegistryMetadata(registriesDir, registryName) // Already returns error
	if err != nil {
		return err
	}
	printRegistryStatus(registryName, registry) // No error return needed, prints to stdout
	return nil
}

// printRegistryStatus displays the registry's package information
func printRegistryStatus(registryName string, registry types.Registry) {
	fmt.Printf("Registry Status for '%s':\n", registryName)
	if len(registry.Packages) == 0 {
		fmt.Println("  No packages registered.")
	} else {
		fmt.Println("  Packages:")
		for pkgName, pkgUUID := range registry.Packages {
			fmt.Printf("    - %s (UUID: %s)\n", pkgName, pkgUUID)
		}
	}
}
