package commands

import (
	"bufio"
	"cosm/types"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// setupRegistriesDir constructs the registries directory path
func setupRegistriesDir(cosmDir string) string {
	return filepath.Join(cosmDir, "registries")
}

// getCosmDir returns the .cosm directory path from COSM_DEPOT_PATH, or an error if unset
func getCosmDir() (string, error) {
	if depotPath := os.Getenv("COSM_DEPOT_PATH"); depotPath != "" {
		return depotPath, nil
	}
	return "", fmt.Errorf("COSM_DEPOT_PATH environment variable is not set")
}

// getRegistriesDir returns the registries directory within the .cosm directory
func getRegistriesDir() (string, error) {
	cosmDir, err := getCosmDir()
	if err != nil {
		return "", fmt.Errorf("failed to get cosm directory: %v", err)
	}
	registriesDir := filepath.Join(cosmDir, "registries")
	if err := os.MkdirAll(registriesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create registries directory %s: %v", registriesDir, err)
	}
	return registriesDir, nil
}

// initializeCosmDir sets up the .cosm directory with essential files and folders
func InitializeCosm() error {

	validDepotVar := verifyCosmDepotVar()
	validDepotDir := verifyCosmDepot()

	// If COSM_DEPOT_PATH is set and the direcory is valid, skip initialization
	if validDepotVar && validDepotDir {
		return nil
	}

	if !validDepotVar {
		if err := initializeCosmDepotVar(); err != nil {
			return err
		}
	}

	if !validDepotDir {
		if err := initializeCosmDepot(); err != nil {
			return err
		}
	}

	return nil
}

// verifyCosmDepot checks if COSM_DEPOT_PATH is set and verifies the .cosm directory structure
func verifyCosmDepotVar() bool {
	depotPath := os.Getenv("COSM_DEPOT_PATH")
	return depotPath != ""
}

// verifyCosmDepot checks if COSM_DEPOT_PATH is set and verifies the .cosm directory structure
func verifyCosmDepot() bool {
	depotPath := os.Getenv("COSM_DEPOT_PATH")
	if depotPath == "" {
		return false
	}

	// Verify directory exists
	if _, err := os.Stat(depotPath); err != nil {
		return false
	}

	// Verify required subdirectories
	requiredDirs := []string{
		"registries",
		"templates",
		"clones",
		"packages",
	}
	for _, dir := range requiredDirs {
		dirPath := filepath.Join(depotPath, dir)
		if _, err := os.Stat(dirPath); err != nil {
			return false
		}
	}

	// Verify registries.json
	registriesFile := filepath.Join(depotPath, "registries", "registries.json")
	if _, err := os.Stat(registriesFile); err != nil {
		return false
	}

	return true
}

// initializeCosmDepot prompts for and sets COSM_DEPOT_PATH if unset or invalid, updating the shell profile
func initializeCosmDepotVar() error {

	// Get default .cosm path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}
	defaultPath := filepath.Join(homeDir, ".cosm")

	// Prompt for location
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("COSM_DEPOT_PATH is not set or invalid. Enter the location for the .cosm directory (default: %s): ", defaultPath)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %v", err)
	}
	input = strings.TrimSpace(input)

	// Use default if input is empty
	depotPath := defaultPath
	if input != "" {
		depotPath = filepath.Clean(input)
		if !filepath.IsAbs(depotPath) {
			depotPath, err = filepath.Abs(depotPath)
			if err != nil {
				return fmt.Errorf("failed to resolve absolute path for %s: %v", input, err)
			}
		}
	}

	// Check if depotPath already exists
	if _, err := os.Stat(depotPath); !os.IsNotExist(err) {
		if err != nil {
			return fmt.Errorf("failed to check if %s exists: %v", depotPath, err)
		}
		return fmt.Errorf("directory %s already exists; please choose a new location", depotPath)
	}

	// Set COSM_DEPOT_PATH for the current process
	if err := os.Setenv("COSM_DEPOT_PATH", depotPath); err != nil {
		return fmt.Errorf("failed to set COSM_DEPOT_PATH: %v", err)
	}

	// Create the cosm depot path
	if err := os.MkdirAll(depotPath, 0755); err != nil {
		return fmt.Errorf("failed to create cosm depot path %s: %v", depotPath, err)
	}

	// Update shell profile
	if err := updateShellProfile(depotPath); err != nil {
		return fmt.Errorf("failed to update shell profile: %v", err)
	}

	// Print confirmation with export instruction
	fmt.Printf("COSM_DEPOT_PATH set to %s and added to shell profile\n", depotPath)
	fmt.Printf("To apply COSM_DEPOT_PATH in the current session, run: export COSM_DEPOT_PATH=%q\n", depotPath)
	return nil
}

// initializeCosmDir sets up the .cosm directory with essential files and folders
func initializeCosmDepot() error {

	// get the cosm depot path
	cosmDir, err := getCosmDir()
	if err != nil {
		return fmt.Errorf("failed to get cosm directory: %v", err)
	}

	// Create registries directory
	registriesDir := setupRegistriesDir(cosmDir)
	if err := os.MkdirAll(registriesDir, 0755); err != nil {
		return fmt.Errorf("failed to create registries directory %s: %v", registriesDir, err)
	}

	// Create empty registries.json if it doesn't exist
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if _, err := os.Stat(registriesFile); os.IsNotExist(err) {
		if err := os.WriteFile(registriesFile, []byte("[]"), 0644); err != nil {
			return fmt.Errorf("failed to create registries.json: %v", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to stat registries.json: %v", err)
	}

	// Create and initialize templates directory
	templatesDir := filepath.Join(cosmDir, "templates")
	// Clone cosm-templates repository
	if _, err := clone("https://github.com/simkinetic/cosm-templates.git", templatesDir); err != nil {
		return fmt.Errorf("failed to clone cosm-templates repository: %v", err)
	}

	// Create clones directory
	clonesDir := filepath.Join(cosmDir, "clones")
	if err := os.MkdirAll(clonesDir, 0755); err != nil {
		return fmt.Errorf("failed to create clones directory %s: %v", clonesDir, err)
	}

	// Create packages directory
	packagesDir := filepath.Join(cosmDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create packages directory %s: %v", packagesDir, err)
	}

	return nil
}

// updateShellProfile appends the COSM_DEPOT_PATH export to the user's shell profile
func updateShellProfile(depotPath string) error {
	profilePath, err := getShellProfilePath()
	if err != nil {
		return err
	}

	// Check if COSM_DEPOT_PATH is already set in the profile
	content, err := os.ReadFile(profilePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read shell profile %s: %v", profilePath, err)
	}
	if strings.Contains(string(content), "export COSM_DEPOT_PATH=") {
		return nil // Already set
	}

	// Append export statement
	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open shell profile %s: %v", profilePath, err)
	}
	defer f.Close()
	if _, err := f.WriteString(fmt.Sprintf("\nexport COSM_DEPOT_PATH=%q\n", depotPath)); err != nil {
		return fmt.Errorf("failed to write to shell profile %s: %v", profilePath, err)
	}

	return nil
}

// getShellProfilePath determines the appropriate shell profile file (.bash_profile or .zprofile)
func getShellProfilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %v", err)
	}

	// Check shell type
	shell := os.Getenv("SHELL")
	var profilePath string
	if strings.Contains(shell, "zsh") {
		profilePath = filepath.Join(homeDir, ".zprofile")
	} else {
		profilePath = filepath.Join(homeDir, ".bash_profile")
	}

	return profilePath, nil
}

// loadRegistryNames loads the list of registry names from registries.json
func loadRegistryNames(registriesDir string) ([]string, error) {
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if _, err := os.Stat(registriesFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no registries found (run 'cosm registry init' first)")
	}
	data, err := os.ReadFile(registriesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read registries.json: %v", err)
	}
	var registryNames []string
	if err := json.Unmarshal(data, &registryNames); err != nil {
		return nil, fmt.Errorf("failed to parse registries.json: %v", err)
	}
	if len(registryNames) == 0 {
		return nil, fmt.Errorf("no registries available to search for packages")
	}
	return registryNames, nil
}

// LoadRegistryMetadata loads and validates the registry metadata from registry.json
func LoadRegistryMetadata(registriesDir, registryName string) (types.Registry, string, error) {
	registryMetaFile := filepath.Join(registriesDir, registryName, "registry.json")
	data, err := os.ReadFile(registryMetaFile)
	if err != nil {
		return types.Registry{}, "", fmt.Errorf("failed to read registry.json for '%s': %v", registryName, err)
	}
	var registry types.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return types.Registry{}, "", fmt.Errorf("failed to parse registry.json for '%s': %v", registryName, err)
	}
	if registry.Packages == nil {
		registry.Packages = make(map[string]types.PackageInfo)
	}
	return registry, registryMetaFile, nil
}

// ensureProjectFileDoesNotExist checks if Project.json already exists
func ensureProjectFileDoesNotExist(projectFile string) error {
	if _, err := os.Stat(projectFile); !os.IsNotExist(err) {
		return fmt.Errorf("Project.json already exists in this directory")
	}
	return nil
}

// loadProject loads and parses Project.json from the specified file path.
func loadProject(filename string) (*types.Project, error) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("no Project.json found at %s", filename)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read Project.json at %s: %v", filename, err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("failed to parse Project.json at %s: %v", filename, err)
	}
	if project.Deps == nil {
		project.Deps = make(map[string]types.Dependency)
	}
	return &project, nil
}

// loadProjectFromDir loads and parses Project.json from the specified directory.
func loadProjectFromDir(dir string) (*types.Project, error) {
	return loadProject(filepath.Join(dir, "Project.json"))
}

// saveProject marshals the project to JSON and writes it to Project.json
func saveProject(project *types.Project, filename string) error {
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %v", filename, err)
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %v", filename, err)
	}
	return nil
}

// saveRegistryNames marshals and writes the list of registry names to registries.json
func saveRegistryNames(registryNames []string, registriesDir string) error {
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

// saveRegistryMetadata marshals and writes the registry metadata to registry.json
func saveRegistryMetadata(registry types.Registry, filename string) error {
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry.json: %v", err)
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %v", filename, err)
	}
	return nil
}

// savePackageVersions marshals and writes the list of package versions to versions.json
func savePackageVersions(versions []string, versionsFile string) error {
	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %v", versionsFile, err)
	}
	if err := os.WriteFile(versionsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %v", versionsFile, err)
	}
	return nil
}

// loadVersions loads the list of versions for a package from versions.json
func loadVersions(registriesDir, registryName, packageName string) ([]string, error) {
	versionsFile := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName, "versions.json")
	data, err := os.ReadFile(versionsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read versions.json for '%s' in registry '%s': %v", packageName, registryName, err)
	}
	var versions []string
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("failed to parse versions.json for '%s' in registry '%s': %v", packageName, registryName, err)
	}
	return versions, nil
}

// loadSpecs loads a package's specs from specs.json
func loadSpecs(registriesDir, registryName, packageName, version string) (types.Specs, error) {
	specsFile := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName, version, "specs.json")
	data, err := os.ReadFile(specsFile)
	if err != nil {
		return types.Specs{}, fmt.Errorf("failed to read specs.json: %v", err)
	}
	var specs types.Specs
	if err := json.Unmarshal(data, &specs); err != nil {
		return types.Specs{}, fmt.Errorf("failed to parse specs.json: %v", err)
	}
	return specs, nil
}

// loadBuildList loads a package's build list from buildlist.json
func loadBuildList(registriesDir, registryName, packageName, version string) (types.BuildList, error) {
	buildListFile := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName, version, "buildlist.json")
	return loadBuildListFile(buildListFile)
}

// loadBuildList loads a package's build list from buildlist.json
func loadBuildListFile(buildListFile string) (types.BuildList, error) {
	data, err := os.ReadFile(buildListFile)
	if err != nil {
		if os.IsNotExist(err) {
			return types.BuildList{Dependencies: make(map[string]types.BuildListDependency)}, nil // No build list yet
		}
		return types.BuildList{}, fmt.Errorf("failed to read buildlist.json: %v", err)
	}
	var buildList types.BuildList
	if err := json.Unmarshal(data, &buildList); err != nil {
		return types.BuildList{}, fmt.Errorf("failed to parse buildlist.json: %v", err)
	}
	return buildList, nil
}

// copyFile copies a single file from src to dest using io.Copy
func copyFile(src, dest string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %v", src, err)
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %v", dest, err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %v", src, dest, err)
	}

	// Ensure the destination file has the same permissions as the source
	if err := destFile.Chmod(mode); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %v", dest, err)
	}

	return nil
}
