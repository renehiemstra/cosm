package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func RegistryRm(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	if len(args) < 2 || len(args) > 3 {
		return fmt.Errorf("requires registry name and package name, with optional version")
	}
	registryName, packageName := args[0], args[1]
	versionTag := ""
	if len(args) == 3 {
		versionTag = args[2]
	}
	registriesDir, err := getRegistriesDir()
	if err != nil {
		return err
	}
	if err := updateSingleRegistry(registriesDir, registryName); err != nil {
		return err
	}
	registry, registryMetaFile, err := LoadRegistryMetadata(registriesDir, registryName)
	if err != nil {
		return err
	}
	if _, exists := registry.Packages[packageName]; !exists {
		return fmt.Errorf("package '%s' not found in registry '%s'", packageName, registryName)
	}
	if versionTag != "" {
		packageDir := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName)
		versionDir := filepath.Join(packageDir, versionTag)
		versionsFile := filepath.Join(packageDir, "versions.json")
		if _, err := os.Stat(versionDir); os.IsNotExist(err) {
			return fmt.Errorf("version '%s' not found for package '%s' in registry '%s'", versionTag, packageName, registryName)
		}
		var versions []string
		data, err := os.ReadFile(versionsFile)
		if err != nil {
			return fmt.Errorf("failed to read versions.json for package '%s': %v", packageName, err)
		}
		if err := json.Unmarshal(data, &versions); err != nil {
			return fmt.Errorf("failed to parse versions.json for package '%s': %v", packageName, err)
		}
		if !contains(versions, versionTag) {
			return fmt.Errorf("version '%s' not found in versions.json for package '%s'", versionTag, packageName)
		}
		if !force {
			prompt := fmt.Sprintf("Are you sure you want to remove version '%s' of package '%s' from registry '%s'? [y/N]: ", versionTag, packageName, registryName)
			if !promptUserForConfirmation(prompt) {
				return fmt.Errorf("operation cancelled by user")
			}
		}
		if err := os.RemoveAll(versionDir); err != nil {
			return fmt.Errorf("failed to remove version '%s' of package '%s': %v", versionTag, packageName, err)
		}
		versions = removeString(versions, versionTag)
		data, err = json.MarshalIndent(versions, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal versions.json for package '%s': %v", packageName, err)
		}
		if err := os.WriteFile(versionsFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write versions.json for package '%s': %v", packageName, err)
		}
		commitMsg := fmt.Sprintf("Removed version '%s' of package '%s'", versionTag, packageName)
		if err := commitAndPushRegistryChanges(registriesDir, registryName, commitMsg); err != nil {
			return err
		}
		fmt.Printf("Removed version '%s' of package '%s' from registry '%s'\n", versionTag, packageName, registryName)
		return nil
	}
	if !force {
		prompt := fmt.Sprintf("Are you sure you want to remove package '%s' from registry '%s'? [y/N]: ", packageName, registryName)
		if !promptUserForConfirmation(prompt) {
			return fmt.Errorf("operation cancelled by user")
		}
	}
	packageDir := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName)
	if err := os.RemoveAll(packageDir); err != nil {
		return fmt.Errorf("failed to remove package '%s': %v", packageName, err)
	}
	delete(registry.Packages, packageName)
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry.json for '%s': %v", registryName, err)
	}
	if err := os.WriteFile(registryMetaFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry.json for '%s': %v", registryName, err)
	}
	commitMsg := fmt.Sprintf("Removed package '%s'", packageName)
	if err := commitAndPushRegistryChanges(registriesDir, registryName, commitMsg); err != nil {
		return err
	}
	fmt.Printf("Removed package '%s' from registry '%s'\n", packageName, registryName)
	return nil
}
