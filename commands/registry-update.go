package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// RegistryUpdate updates and synchronizes a registry or all registries with their remotes
func RegistryUpdate(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	if all && len(args) != 0 {
		return fmt.Errorf("no arguments allowed with --all flag")
	}
	if !all && len(args) != 1 {
		return fmt.Errorf("exactly one argument required (e.g., cosm registry update <registry_name>)")
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return err
	}
	registriesDir := setupRegistriesDir(cosmDir)

	if all {
		registryNames, err := loadRegistryNames(cosmDir)
		if err != nil {
			return fmt.Errorf("failed to load registry names: %v", err)
		}
		if len(registryNames) == 0 {
			fmt.Println("No registries to update.")
			return nil
		}
		for _, name := range registryNames {
			if err := updateSingleRegistry(registriesDir, name); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to update registry '%s': %v\n", name, err)
				continue
			}
			fmt.Printf("Updated registry '%s'\n", name)
		}
		return nil
	}

	registryName := args[0]
	if err := updateSingleRegistry(registriesDir, registryName); err != nil {
		return err
	}
	fmt.Printf("Updated registry '%s'\n", registryName)
	return nil
}
