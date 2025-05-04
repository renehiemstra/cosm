package commands

import (
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

// getGlobalCosmDir returns the global .cosm directory in the user's home directory
func getGlobalCosmDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %v", err)
	}
	return filepath.Join(homeDir, ".cosm"), nil
}

// getCosmDir retrieves the global .cosm directory
func getCosmDir() (string, error) {
	cosmDir, err := getGlobalCosmDir()
	if err != nil {
		return "", fmt.Errorf("failed to get global .cosm directory: %v", err)
	}
	return cosmDir, nil
}

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

// loadRegistryNames loads the list of registry names from registries.json
func loadRegistryNames(cosmDir string) ([]string, error) {
	registriesDir := filepath.Join(cosmDir, "registries")
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

// loadProjectFile loads and parses Project.json from the specified directory
func loadProjectFile(dir string) (types.Project, error) {
	projectFile := filepath.Join(dir, "Project.json")
	data, err := os.ReadFile(projectFile)
	if err != nil {
		return types.Project{}, fmt.Errorf("failed to read Project.json at %s: %v", projectFile, err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return types.Project{}, fmt.Errorf("failed to parse Project.json at %s: %v", projectFile, err)
	}
	if project.Deps == nil {
		project.Deps = make(map[string]string)
	}
	return project, nil
}

// loadProject reads and parses the Project.json file
func loadProject(projectFile string) (*types.Project, error) {
	if _, err := os.Stat(projectFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no Project.json found in current directory")
	}
	data, err := os.ReadFile(projectFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read Project.json: %v", err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("failed to parse Project.json: %v", err)
	}
	if project.Deps == nil {
		project.Deps = make(map[string]string)
	}
	return &project, nil
}

// marshalProject converts the project struct to JSON
func marshalProject(project types.Project) ([]byte, error) {
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Project.json: %v", err) // Return error
	}
	return data, nil
}

// writeProjectFile writes the project data to Project.json
func writeProjectFile(projectFile string, data []byte) error {
	if err := os.WriteFile(projectFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write Project.json: %v", err) // Return error
	}
	return nil
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
