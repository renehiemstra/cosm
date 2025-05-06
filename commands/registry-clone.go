package commands

import (
	"bufio"
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// cloneRegistryConfig holds configuration for cloning a registry
type cloneRegistryConfig struct {
	gitURL        string
	cosmDir       string
	registriesDir string
	registry      types.Registry
	registryNames []string
	registryPath  string
}

// RegistryClone clones a registry from a Git URL to the registries directory
func RegistryClone(cmd *cobra.Command, args []string) error {
	// Parse arguments and initialize config
	config, err := parseCloneArgs(args)
	if err != nil {
		return err
	}

	// Load existing registry names
	config.registryNames, err = loadRegistryNames(config.registriesDir)
	if err != nil {
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no registries available") {
			return fmt.Errorf("failed to load registry names: %v", err)
		}
		config.registryNames = []string{} // Initialize empty list if registries.json is missing or empty
	}

	// Validate cloned registry
	if err := validateClonedRegistry(config); err != nil {
		return err
	}

	// Prompt for overwrite if registry exists
	if err := promptForOverwrite(config); err != nil {
		return err
	}

	// Clone to final location and update registries.json
	if err := cloneRegistryToFinalLocation(config); err != nil {
		return err
	}

	fmt.Printf("Cloned registry '%s' from %s\n", config.registry.Name, config.gitURL)
	return nil
}

// parseCloneArgs validates the Git URL argument and initializes the config
func parseCloneArgs(args []string) (*cloneRegistryConfig, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("exactly one argument required (e.g., cosm registry clone <giturl>)")
	}
	gitURL := args[0]
	if gitURL == "" {
		return nil, fmt.Errorf("git URL cannot be empty")
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get cosm directory: %v", err)
	}
	registriesDir, err := getRegistriesDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get registries directory: %v", err)
	}

	return &cloneRegistryConfig{
		gitURL:        gitURL,
		cosmDir:       cosmDir,
		registriesDir: registriesDir,
	}, nil
}

// validateClonedRegistry clones to a temporary directory and validates registry.json
func validateClonedRegistry(config *cloneRegistryConfig) error {
	tmpDir, err := os.MkdirTemp("", "cosm-clone-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := clone(config.gitURL, tmpDir); err != nil {
		msg := fmt.Sprintf("failed to clone repository at '%s' to %s", config.gitURL, tmpDir)
		return wrapGitError(filepath.Dir(tmpDir), msg, err)
	}

	registryMetaFile := filepath.Join(tmpDir, "registry.json")
	data, err := os.ReadFile(registryMetaFile)
	if err != nil {
		return fmt.Errorf("failed to read %s from cloned repository: %v", registryMetaFile, err)
	}
	if err := json.Unmarshal(data, &config.registry); err != nil {
		return fmt.Errorf("failed to parse %s: %v", registryMetaFile, err)
	}
	if config.registry.Name == "" {
		return fmt.Errorf("%s does not contain a valid registry name", registryMetaFile)
	}
	config.registryPath = filepath.Join(config.registriesDir, config.registry.Name)
	return nil
}

// promptForOverwrite prompts the user and deletes an existing registry if needed
func promptForOverwrite(config *cloneRegistryConfig) error {
	if contains(config.registryNames, config.registry.Name) {
		fmt.Printf("Registry '%s' already exists. Overwrite? [y/N]: ", config.registry.Name)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if response != "y" && response != "yes" {
			fmt.Println("Registry clone cancelled.")
			return nil
		}
		// Delete existing registry (without --force to align with prompt)
		deleteCmd := &cobra.Command{}
		if err := RegistryDelete(deleteCmd, []string{config.registry.Name}); err != nil {
			return fmt.Errorf("failed to delete existing registry '%s': %v", config.registry.Name, err)
		}
	}
	return nil
}

// cloneRegistryToFinalLocation clones the registry to its final path and updates registries.json
func cloneRegistryToFinalLocation(config *cloneRegistryConfig) error {
	if _, err := clone(config.gitURL, config.registryPath); err != nil {
		msg := fmt.Sprintf("failed to clone repository to '%s'", config.registryPath)
		return wrapGitError(filepath.Dir(config.registryPath), msg, err)
	}

	// Update registries.json
	config.registryNames = append(config.registryNames, config.registry.Name)
	if err := saveRegistryNames(config.registryNames, config.registriesDir); err != nil {
		return err
	}
	return nil
}
