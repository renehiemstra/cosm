package commands

import (
	"cosm/types"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Add adds a dependency to the project's Project.json file
func Add(cmd *cobra.Command, args []string) error {
	packageName, versionTag, err := parseAddArgs(args)
	if err != nil {
		return err
	}
	project, err := loadProject("Project.json")
	if err != nil {
		return err
	}
	registriesDir, err := getRegistriesDir()
	if err != nil {
		return err
	}
	registryNames, err := loadRegistryNames(registriesDir)
	if err != nil {
		return err
	}
	selectedPackage, err := findPackageInRegistries(packageName, versionTag, registriesDir, registryNames)
	if err != nil {
		return err
	}
	if err := updateProjectWithDependency(project, packageName, selectedPackage.Specs.Version, selectedPackage.RegistryName, selectedPackage.Specs.UUID); err != nil {
		return err
	}
	return nil
}

// parseAddArgs validates and parses the package name and optional version
func parseAddArgs(args []string) (string, string, error) {
	if len(args) < 1 || len(args) > 2 {
		return "", "", fmt.Errorf("expected 1 or 2 arguments in the format <package_name> [v<version_number>] (e.g., cosm add mypkg v1.2.3)")
	}
	packageName := args[0]
	if packageName == "" {
		return "", "", fmt.Errorf("package name cannot be empty")
	}
	versionTag := ""
	if len(args) == 2 {
		versionTag = args[1]
		if !strings.HasPrefix(versionTag, "v") {
			return "", "", fmt.Errorf("version '%s' must start with 'v'", versionTag)
		}
	}
	return packageName, versionTag, nil
}

// updateDependency adds a dependency to the project's Deps map
func updateDependency(project *types.Project, packageName, versionTag, depUUID string) error {
	// Ensure Deps map is initialized
	if project.Deps == nil {
		project.Deps = make(map[string]types.Dependency)
	}

	// Get major version for the key
	majorVersion, err := GetMajorVersion(versionTag)
	if err != nil {
		return fmt.Errorf("failed to get major version for %s@%s: %v", packageName, versionTag, err)
	}

	// Create the dependency key
	depKey := fmt.Sprintf("%s@%s", depUUID, majorVersion)

	// Check if dependency already exists
	if _, exists := project.Deps[depKey]; exists {
		return fmt.Errorf("dependency '%s' with major version %s already exists in project", packageName, majorVersion)
	}

	// Add the dependency
	project.Deps[depKey] = types.Dependency{
		Name:    packageName,
		Version: versionTag,
		Develop: false,
	}
	return nil
}

// updateProjectWithDependency adds the dependency and saves the updated project
func updateProjectWithDependency(project *types.Project, packageName, versionTag, registryName, depUUID string) error {
	if err := updateDependency(project, packageName, versionTag, depUUID); err != nil {
		return err
	}
	if err := saveProject(project, "Project.json"); err != nil {
		return err
	}
	fmt.Printf("Added dependency '%s' %s from registry '%s' to project\n", packageName, versionTag, registryName)
	return nil
}
