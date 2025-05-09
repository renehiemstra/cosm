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
func findPackageInRegistries(packageName, versionTag, registriesDir string, registryNames []string) (types.PackageLocation, error) {
	var foundPackages []types.PackageLocation

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

	if _, exists := registry.Packages[packageName]; !exists {
		return types.PackageLocation{}, false, nil
	}

	// Determine the version to use
	version := versionTag
	if versionTag == "" {
		latestVersion, err := findLatestVersionInRegistry(packageName, registriesDir, registryName)
		if err != nil {
			return types.PackageLocation{}, false, err
		}
		if latestVersion == "" {
			return types.PackageLocation{}, false, nil
		}
		version = latestVersion
	}

	// Load specs for the selected version
	specs, err := loadSpecs(registriesDir, registryName, packageName, version)
	if err != nil {
		if os.IsNotExist(err) {
			return types.PackageLocation{}, false, nil
		}
		return types.PackageLocation{}, false, fmt.Errorf("failed to load specs for '%s@%s' in registry '%s': %v", packageName, version, registryName, err)
	}
	if specs.Version != version {
		return types.PackageLocation{}, false, nil
	}

	return types.PackageLocation{RegistryName: registryName, Specs: specs}, true, nil
}

// findLatestVersionInRegistry finds the latest version of a package in a single registry
func findLatestVersionInRegistry(packageName, registriesDir, registryName string) (string, error) {
	// Load versions
	versions, err := loadVersions(registriesDir, registryName, packageName)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", nil
	}

	// Determine the latest version
	latestVersion, err := determineLatestVersion(versions)
	if err != nil {
		return "", err
	}

	return latestVersion, nil
}

// determineLatestVersion finds the latest version from a list of versions
func determineLatestVersion(versions []string) (string, error) {
	var latestVersion string

	for _, version := range versions {
		if latestVersion == "" {
			latestVersion = version
		} else {
			maxVersion, err := MaxSemVer(latestVersion, version)
			if err != nil {
				continue // Skip invalid versions
			}
			if maxVersion == version {
				latestVersion = version
			}
		}
	}

	return latestVersion, nil
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

// commitAndPushRegistryChanges stages, commits, and pushes changes to the registry
func commitAndPushRegistryChanges(registriesDir, registryName, commitMsg string) error {
	registryDir := filepath.Join(registriesDir, registryName)

	// Stage all changes
	if err := stageFiles(registryDir, "."); err != nil {
		return err
	}

	// Commit changes
	if err := commitChanges(registryDir, commitMsg); err != nil {
		return err
	}

	// Get the current branch
	branch, err := getCurrentBranch(registryDir)
	if err != nil {
		return err
	}

	// Push changes to the current branch
	return pushToRemote(registryDir, branch, false)
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
