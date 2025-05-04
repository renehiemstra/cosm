package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// RegistryInit initializes a new package registry
func RegistryInit(cmd *cobra.Command, args []string) error {
	originalDir, registryName, gitURL, registriesDir, err := setupAndParseInitArgs(args) // Updated to handle error
	if err != nil {
		return err
	}
	registryNames, err := loadAndCheckRegistries(registriesDir, registryName) // Updated to handle error
	if err != nil {
		return err
	}
	registrySubDir, err := cloneAndEnterRegistry(registriesDir, registryName, gitURL) // Updated to handle error
	if err != nil {
		return err
	}
	if err := ensureDirectoryEmpty(registrySubDir, gitURL); err != nil { // Updated to handle error
		cleanupInit(originalDir, registrySubDir, true)
		return err
	}
	if err := updateRegistriesList(registriesDir, registryNames, registryName); err != nil { // Updated to handle error
		cleanupInit(originalDir, registrySubDir, true)
		return err
	}
	_, err = initializeRegistryMetadata(registrySubDir, registryName, gitURL) // Updated to handle error
	if err != nil {
		cleanupInit(originalDir, registrySubDir, true)
		return err
	}
	if err := commitAndPushInitialRegistryChanges(registryName, gitURL); err != nil { // Updated to handle error
		cleanupInit(originalDir, registrySubDir, true)
		return err
	}
	if err := restoreOriginalDir(originalDir); err != nil { // Updated to handle error
		return err
	}
	fmt.Printf("Initialized registry '%s' with Git URL: %s\n", registryName, gitURL)
	return nil
}

// setupAndParseInitArgs validates arguments and sets up directories for RegistryInit
func setupAndParseInitArgs(args []string) (string, string, string, string, error) {
	originalDir, err := os.Getwd()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get original directory: %v", err)
	}

	if len(args) != 2 {
		return "", "", "", "", fmt.Errorf("exactly two arguments required (e.g., cosm registry init <registry name> <giturl>)")
	}
	registryName := args[0]
	gitURL := args[1]

	if registryName == "" {
		return "", "", "", "", fmt.Errorf("registry name cannot be empty")
	}
	if gitURL == "" {
		return "", "", "", "", fmt.Errorf("git URL cannot be empty")
	}

	cosmDir, err := getGlobalCosmDir()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get global .cosm directory: %v", err)
	}
	registriesDir := filepath.Join(cosmDir, "registries")
	if err := os.MkdirAll(registriesDir, 0755); err != nil {
		return "", "", "", "", fmt.Errorf("failed to create %s directory: %v", registriesDir, err)
	}

	return originalDir, registryName, gitURL, registriesDir, nil
}

// cleanupInit reverts to the original directory and removes the registrySubDir if needed
func cleanupInit(originalDir, registrySubDir string, removeDir bool) {
	if err := os.Chdir(originalDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error returning to original directory %s: %v\n", originalDir, err)
		// Don’t exit here; let the caller handle the exit after cleanup
	}
	if removeDir {
		if err := os.RemoveAll(registrySubDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to clean up registry directory %s: %v\n", registrySubDir, err)
		}
	}
}

// cloneAndEnterRegistry clones the repository into registries/<registryName> and changes to it
func cloneAndEnterRegistry(registriesDir, registryName, gitURL string) (string, error) {
	registrySubDir := filepath.Join(registriesDir, registryName)
	cloneCmd := exec.Command("git", "clone", gitURL, registrySubDir)
	cloneOutput, err := cloneCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to clone repository at '%s' into %s: %v\nOutput: %s", gitURL, registrySubDir, err, cloneOutput)
	}

	// Change to the cloned directory
	if err := os.Chdir(registrySubDir); err != nil {
		return "", fmt.Errorf("failed to change to registry directory %s: %v", registrySubDir, err)
	}
	return registrySubDir, nil
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

// commitAndPushInitialRegistryChanges stages, commits, and pushes the initial registry changes
func commitAndPushInitialRegistryChanges(registryName, gitURL string) error {
	addCmd := exec.Command("git", "add", "registry.json")
	addOutput, err := addCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stage registry.json: %v\nOutput: %s", err, addOutput)
	}
	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("Initialized registry %s", registryName))
	commitOutput, err := commitCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit initial registry setup: %v\nOutput: %s", err, commitOutput)
	}
	pushCmd := exec.Command("git", "push", "origin", "main")
	pushOutput, err := pushCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push initial commit to %s: %v\nOutput: %s", gitURL, err, pushOutput)
	}
	return nil
}

// restoreOriginalDir returns to the original directory without removing the registry subdir
func restoreOriginalDir(originalDir string) error {
	if err := os.Chdir(originalDir); err != nil {
		return fmt.Errorf("failed to return to original directory %s: %v", originalDir, err)
	}
	return nil
}
