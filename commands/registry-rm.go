package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// rmRegistryConfig holds configuration for removing a package or version from a registry
type rmRegistryConfig struct {
	registryName  string
	packageName   string
	versionTag    string
	registriesDir string
	force         bool
	registry      types.Registry
	registryFile  string
	packageDir    string
	versionDir    string
}

// RegistryRm removes a package or a specific version from a registry
func RegistryRm(cmd *cobra.Command, args []string) error {
	// Parse arguments and initialize config
	config, err := parseRmArgs(cmd, args)
	if err != nil {
		return err
	}

	// Validate registry and package
	if err := validateRegistryAndPackage(config); err != nil {
		return err
	}

	// Prompt for confirmation if not forced
	if err := promptForRm(config); err != nil {
		return err
	}

	// Remove package or version and commit changes
	if config.versionTag != "" {
		return removePackageVersion(config)
	}
	return removeEntirePackage(config)
}

// parseRmArgs parses and validates the registry name, package name, and optional version
func parseRmArgs(cmd *cobra.Command, args []string) (*rmRegistryConfig, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("requires registry name and package name, with optional version (e.g., cosm registry rm <registry> <package> [<version>])")
	}
	registryName, packageName := args[0], args[1]
	versionTag := ""
	if len(args) == 3 {
		versionTag = args[2]
	}
	if registryName == "" {
		return nil, fmt.Errorf("registry name cannot be empty")
	}
	if packageName == "" {
		return nil, fmt.Errorf("package name cannot be empty")
	}
	if versionTag != "" && !strings.HasPrefix(versionTag, "v") {
		return nil, fmt.Errorf("version must start with 'v' if provided")
	}

	registriesDir, err := getRegistriesDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get registries directory: %v", err)
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return nil, fmt.Errorf("failed to get force flag: %v", err)
	}

	config := &rmRegistryConfig{
		registryName:  registryName,
		packageName:   packageName,
		versionTag:    versionTag,
		registriesDir: registriesDir,
		force:         force,
		packageDir:    filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName),
	}
	if versionTag != "" {
		config.versionDir = filepath.Join(config.packageDir, versionTag)
	}

	return config, nil
}

// validateRegistryAndPackage updates the registry and validates the package and version
func validateRegistryAndPackage(config *rmRegistryConfig) error {
	if err := updateSingleRegistry(config.registriesDir, config.registryName); err != nil {
		return fmt.Errorf("failed to update registry '%s': %v", config.registryName, err)
	}

	var err error
	config.registry, config.registryFile, err = LoadRegistryMetadata(config.registriesDir, config.registryName)
	if err != nil {
		return fmt.Errorf("failed to load registry metadata for '%s': %v", config.registryName, err)
	}

	if _, exists := config.registry.Packages[config.packageName]; !exists {
		return fmt.Errorf("package '%s' not found in registry '%s'", config.packageName, config.registryName)
	}

	if config.versionTag != "" {
		if _, err := os.Stat(config.versionDir); os.IsNotExist(err) {
			return fmt.Errorf("version '%s' not found for package '%s' in registry '%s'", config.versionTag, config.packageName, config.registryName)
		}
		versionsFile := filepath.Join(config.packageDir, "versions.json")
		var versions []string
		data, err := os.ReadFile(versionsFile)
		if err != nil {
			return fmt.Errorf("failed to read %s for package '%s': %v", versionsFile, config.packageName, err)
		}
		if err := json.Unmarshal(data, &versions); err != nil {
			return fmt.Errorf("failed to parse %s for package '%s': %v", versionsFile, config.packageName, err)
		}
		if !contains(versions, config.versionTag) {
			return fmt.Errorf("version '%s' not found in %s for package '%s'", config.versionTag, versionsFile, config.packageName)
		}
	}
	return nil
}

// promptForRm prompts the user for confirmation if not forced
func promptForRm(config *rmRegistryConfig) error {
	if !config.force {
		prompt := fmt.Sprintf("Are you sure you want to remove %s from registry '%s'? [y/N]: ",
			getRemovalTarget(config), config.registryName)
		if !promptUserForConfirmation(prompt) {
			return fmt.Errorf("operation cancelled by user")
		}
	}
	return nil
}

// getRemovalTarget returns the description of what is being removed
func getRemovalTarget(config *rmRegistryConfig) string {
	if config.versionTag != "" {
		return fmt.Sprintf("version '%s' of package '%s'", config.versionTag, config.packageName)
	}
	return fmt.Sprintf("package '%s'", config.packageName)
}

// removePackageVersion removes a specific version of a package
func removePackageVersion(config *rmRegistryConfig) error {
	if err := os.RemoveAll(config.versionDir); err != nil {
		return fmt.Errorf("failed to remove directory '%s' for version '%s' of package '%s': %v", config.versionDir, config.versionTag, config.packageName, err)
	}

	versionsFile := filepath.Join(config.packageDir, "versions.json")
	var versions []string
	data, err := os.ReadFile(versionsFile)
	if err != nil {
		return fmt.Errorf("failed to read %s for package '%s': %v", versionsFile, config.packageName, err)
	}
	if err := json.Unmarshal(data, &versions); err != nil {
		return fmt.Errorf("failed to parse %s for package '%s': %v", versionsFile, config.packageName, err)
	}
	versions = removeString(versions, config.versionTag)
	if err := savePackageVersions(versions, versionsFile); err != nil {
		return err
	}

	commitMsg := fmt.Sprintf("Removed version '%s' of package '%s'", config.versionTag, config.packageName)
	if err := commitAndPushRegistryChanges(config.registriesDir, config.registryName, commitMsg); err != nil {
		return fmt.Errorf("failed to commit changes for version '%s' of package '%s': %v", config.versionTag, config.packageName, err)
	}

	fmt.Printf("Removed version '%s' of package '%s' from registry '%s'\n", config.versionTag, config.packageName, config.registryName)
	return nil
}

// removeEntirePackage removes an entire package from the registry
func removeEntirePackage(config *rmRegistryConfig) error {
	if err := os.RemoveAll(config.packageDir); err != nil {
		return fmt.Errorf("failed to remove directory '%s' for package '%s': %v", config.packageDir, config.packageName, err)
	}

	delete(config.registry.Packages, config.packageName)
	if err := saveRegistryMetadata(config.registry, config.registryFile); err != nil {
		return err
	}

	commitMsg := fmt.Sprintf("Removed package '%s'", config.packageName)
	if err := commitAndPushRegistryChanges(config.registriesDir, config.registryName, commitMsg); err != nil {
		return fmt.Errorf("failed to commit changes for package '%s': %v", config.packageName, err)
	}

	fmt.Printf("Removed package '%s' from registry '%s'\n", config.packageName, config.registryName)
	return nil
}
