package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
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
	cosmDir, err := getCosmDir()
	if err != nil {
		return err
	}
	registryNames, err := loadRegistryNames(cosmDir)
	if err != nil {
		return err
	}
	selectedPackage, err := findPackageInRegistries(packageName, versionTag, cosmDir, registryNames)
	if err != nil {
		return err
	}
	if err := updateProjectWithDependency(project, packageName, versionTag, selectedPackage.RegistryName); err != nil {
		return err
	}
	return nil
}

// parseAddArgs validates and parses the package_name@version argument
func parseAddArgs(args []string) (string, string, error) {
	if len(args) != 1 {
		return "", "", fmt.Errorf("exactly one argument required in the format <package_name>@v<version_number> (e.g., cosm add mypkg@v1.2.3)")
	}
	depArg := args[0]
	parts := strings.SplitN(depArg, "@", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("argument '%s' must be in the format <package_name>@v<version_number>", depArg)
	}
	packageName, versionTag := parts[0], parts[1]
	if packageName == "" {
		return "", "", fmt.Errorf("package name cannot be empty")
	}
	if !strings.HasPrefix(versionTag, "v") {
		return "", "", fmt.Errorf("version '%s' must start with 'v'", versionTag)
	}
	return packageName, versionTag, nil
}

// updateProjectWithDependency adds the dependency and saves the updated project
func updateProjectWithDependency(project *types.Project, packageName, versionTag string, registryName string) error {
	if _, exists := project.Deps[packageName]; exists {
		return fmt.Errorf("dependency '%s' already exists in project", packageName)
	}
	project.Deps[packageName] = versionTag

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Project.json: %v", err)
	}
	if err := os.WriteFile("Project.json", data, 0644); err != nil {
		return fmt.Errorf("failed to write Project.json: %v", err)
	}
	fmt.Printf("Added dependency '%s' %s from registry '%s' to project\n", packageName, versionTag, registryName)
	return nil
}
