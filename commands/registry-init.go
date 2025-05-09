package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// RegistryInit initializes a new package registry
func RegistryInit(cmd *cobra.Command, args []string) error {
	registryName, gitURL, registriesDir, err := setupAndParseInitArgs(args)
	if err != nil {
		return err
	}
	registryNames, err := loadAndCheckRegistries(registriesDir, registryName)
	if err != nil {
		return err
	}
	registrySubDir, err := cloneDir(registriesDir, registryName, gitURL)
	if err != nil {
		return err
	}
	if err := ensureDirectoryEmpty(registrySubDir, gitURL); err != nil {
		cleanupInit(registrySubDir)
		return err
	}
	if err := updateRegistriesList(registriesDir, registryNames, registryName); err != nil {
		cleanupInit(registrySubDir)
		return err
	}
	_, err = initializeRegistryMetadata(registrySubDir, registryName, gitURL)
	if err != nil {
		cleanupInit(registrySubDir)
		return err
	}
	if err := commitAndPushInitialRegistryChanges(registryName); err != nil {
		cleanupInit(registrySubDir)
		return err
	}
	fmt.Printf("Initialized registry '%s' with Git URL: %s\n", registryName, gitURL)
	return nil
}

// setupAndParseInitArgs validates arguments and sets up directories for RegistryInit
func setupAndParseInitArgs(args []string) (string, string, string, error) {
	if len(args) != 2 {
		return "", "", "", fmt.Errorf("exactly two arguments required (e.g., cosm registry init <registry name> <giturl>)")
	}
	registryName := args[0]
	gitURL := args[1]

	if registryName == "" {
		return "", "", "", fmt.Errorf("registry name cannot be empty")
	}
	if gitURL == "" {
		return "", "", "", fmt.Errorf("git URL cannot be empty")
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get global .cosm directory: %v", err)
	}
	registriesDir := filepath.Join(cosmDir, "registries")
	if err := os.MkdirAll(registriesDir, 0755); err != nil {
		return "", "", "", fmt.Errorf("failed to create %s directory: %v", registriesDir, err)
	}

	return registryName, gitURL, registriesDir, nil
}

// cleanupInit reverts to the original directory and removes the registrySubDir if needed
func cleanupInit(registrySubDir string) {
	if err := os.RemoveAll(registrySubDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to clean up registry directory %s: %v\n", registrySubDir, err)
	}
}

// ensureDirectoryEmpty checks if the cloned directory is empty except for .git
func ensureDirectoryEmpty(dir, gitURL string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %v", dir, err)
	}
	for _, file := range files {
		if file.Name() != ".git" { // Ignore .git directory
			return fmt.Errorf("repository at '%s' cloned into %s is not empty (contains %s)", gitURL, dir, file.Name())
		}
	}
	return nil
}

// cloneDir clones the repository into registries/<registryName> and returns the directory path.
func cloneDir(registriesDir, registryName, gitURL string) (string, error) {
	return clone(gitURL, registriesDir, registryName)
}

// updateRegistriesList adds the registry name to registries.json
func updateRegistriesList(registriesDir string, registryNames []string, registryName string) error {
	registryNames = append(registryNames, registryName)
	data, err := json.MarshalIndent(registryNames, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registries.json: %v", err)
	}
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if err := os.WriteFile(registriesFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registries.json: %v", err)
	}
	return nil
}

// initializeRegistryMetadata creates and writes the registry.json file
func initializeRegistryMetadata(registrySubDir, registryName, gitURL string) (string, error) {
	registryMetaFile := filepath.Join(registrySubDir, "registry.json")
	registry := types.Registry{
		Name:     registryName,
		UUID:     uuid.New().String(),
		GitURL:   gitURL,
		Packages: make(map[string]types.PackageInfo),
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal registry.json: %v", err)
	}
	if err := os.WriteFile(registryMetaFile, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write registry.json: %v", err)
	}
	return registryMetaFile, nil
}
