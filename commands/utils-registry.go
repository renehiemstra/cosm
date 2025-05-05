package commands

import (
	"bufio"
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// promptUserForRegistry handles multiple registry matches by prompting the user
func promptUserForRegistry(packageName, versionTag string, foundPackages []types.PackageLocation) (types.PackageLocation, error) {
	fmt.Printf("Package '%s' v%s found in multiple registries:\n", packageName, versionTag)
	for i, pkg := range foundPackages {
		fmt.Printf("  %d. %s (Git URL: %s)\n", i+1, pkg.RegistryName, pkg.Specs.GitURL)
	}
	fmt.Printf("Please select a registry (enter number 1-%d): ", len(foundPackages))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())
	choiceNum := 0
	_, err := fmt.Sscanf(choice, "%d", &choiceNum)
	if err != nil || choiceNum < 1 || choiceNum > len(foundPackages) {
		return types.PackageLocation{}, fmt.Errorf("invalid selection '%s': must be a number between 1 and %d", choice, len(foundPackages))
	}
	return foundPackages[choiceNum-1], nil
}

// findPackageInRegistries searches for a package across all registries
func findPackageInRegistries(packageName, versionTag, cosmDir string, registryNames []string) (types.PackageLocation, error) {
	var foundPackages []types.PackageLocation
	registriesDir := filepath.Join(cosmDir, "registries")

	for _, regName := range registryNames {
		pkg, found, err := findPackageInRegistry(packageName, versionTag, registriesDir, regName)
		if err != nil {
			return types.PackageLocation{}, err
		}
		if found {
			foundPackages = append(foundPackages, pkg)
		}
	}

	return selectPackageFromResults(packageName, versionTag, foundPackages)
}

// findPackageInRegistry searches for a package in a single registry
func findPackageInRegistry(packageName, versionTag, registriesDir, registryName string) (types.PackageLocation, bool, error) {
	// Update registry before loading metadata
	if err := updateSingleRegistry(registriesDir, registryName); err != nil {
		return types.PackageLocation{}, false, err
	}
	registry, _, err := LoadRegistryMetadata(registriesDir, registryName)
	if err != nil {
		return types.PackageLocation{}, false, fmt.Errorf("failed to load registry metadata for '%s': %v", registryName, err)
	}

	_, exists := registry.Packages[packageName]
	if !exists {
		return types.PackageLocation{}, false, nil
	}

	specsFile := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName, versionTag, "specs.json")
	if _, err := os.Stat(specsFile); os.IsNotExist(err) {
		return types.PackageLocation{}, false, nil
	}
	data, err := os.ReadFile(specsFile)
	if err != nil {
		return types.PackageLocation{}, false, fmt.Errorf("failed to read specs.json for '%s' in registry '%s': %v", packageName, registryName, err)
	}
	var specs types.Specs
	if err := json.Unmarshal(data, &specs); err != nil {
		return types.PackageLocation{}, false, fmt.Errorf("failed to parse specs.json for '%s' in registry '%s': %v", packageName, registryName, err)
	}
	if specs.Version != versionTag {
		return types.PackageLocation{}, false, nil
	}
	return types.PackageLocation{RegistryName: registryName, Specs: specs}, true, nil
}

// updateRegistryConfig holds configuration for updating a registry
type updateRegistryConfig struct {
	registryName  string
	registriesDir string
	registryDir   string
}

// updateSingleRegistry pulls updates for a single registry
func updateSingleRegistry(registriesDir, registryName string) error {
	// Parse arguments and initialize config
	config, err := parseUpdateArgs(registriesDir, registryName)
	if err != nil {
		return err
	}

	// Validate registry existence
	if err := validateRegistryForUpdate(config); err != nil {
		return err
	}

	// Pull updates from the registry's Git repository
	if err := pullRegistryUpdates(config); err != nil {
		return err
	}

	return nil
}

// parseUpdateArgs validates the registry name and initializes the config
func parseUpdateArgs(registriesDir, registryName string) (*updateRegistryConfig, error) {
	if registryName == "" {
		return nil, fmt.Errorf("registry name cannot be empty")
	}
	if registriesDir == "" {
		return nil, fmt.Errorf("registries directory cannot be empty")
	}

	registryDir := filepath.Join(registriesDir, registryName)
	return &updateRegistryConfig{
		registryName:  registryName,
		registriesDir: registriesDir,
		registryDir:   registryDir,
	}, nil
}

// validateRegistryForUpdate checks if the registry exists
func validateRegistryForUpdate(config *updateRegistryConfig) error {
	if err := assertRegistryExists(config.registriesDir, config.registryName); err != nil {
		return fmt.Errorf("failed to validate registry '%s': %v", config.registryName, err)
	}
	return nil
}

// pullRegistryUpdates pulls updates from the current branch of the registry's Git repository
func pullRegistryUpdates(config *updateRegistryConfig) error {
	branch, err := getCurrentBranch(config.registryDir)
	if err != nil {
		return fmt.Errorf("failed to get current branch for registry '%s' in %s: %v", config.registryName, config.registryDir, err)
	}
	context := fmt.Sprintf("registry '%s' in %s", config.registryName, config.registryDir)
	if err := pullFromBranch(config.registryDir, branch, context); err != nil {
		return err
	}
	return nil
}

// assertRegistryExists verifies that the specified registry exists in registries.json
func assertRegistryExists(registriesDir, registryName string) error {
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if _, err := os.Stat(registriesFile); os.IsNotExist(err) {
		return fmt.Errorf("no registries found (run 'cosm registry init' first)")
	}
	var registryNames []string
	data, err := os.ReadFile(registriesFile)
	if err != nil {
		return fmt.Errorf("failed to read registries.json: %v", err)
	}
	if err := json.Unmarshal(data, &registryNames); err != nil {
		return fmt.Errorf("failed to parse registries.json: %v", err)
	}
	for _, name := range registryNames {
		if name == registryName {
			return nil
		}
	}
	return fmt.Errorf("registry '%s' not found in registries.json", registryName)
}

// loadAndCheckRegistries loads registries.json and checks for duplicate registry names
func loadAndCheckRegistries(registriesDir, registryName string) ([]string, error) {
	registriesFile := filepath.Join(registriesDir, "registries.json")
	var registryNames []string
	if data, err := os.ReadFile(registriesFile); err == nil {
		if err := json.Unmarshal(data, &registryNames); err != nil {
			return nil, fmt.Errorf("failed to parse registries.json: %v", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read registries.json: %v", err)
	}

	for _, name := range registryNames {
		if name == registryName {
			return nil, fmt.Errorf("registry '%s' already exists", registryName)
		}
	}

	return registryNames, nil
}
