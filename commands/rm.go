package commands

import (
	"bufio"
	"cosm/types"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Rm removes a dependency from the project's Project.json file
func Rm(cmd *cobra.Command, args []string) error {
	packageName, err := parseRmArgs(args)
	if err != nil {
		return err
	}

	project, err := loadProject("Project.json")
	if err != nil {
		return err
	}

	keys, deps, err := findDependencyKey(project, packageName)
	if err != nil {
		return err
	}

	var depKey string
	if len(keys) > 1 {
		depKey, err = promptUserForDependency(packageName, keys, deps)
		if err != nil {
			return err
		}
	} else {
		depKey = keys[0]
	}

	if err := removeDependency(project, depKey, packageName); err != nil {
		return err
	}

	return nil
}

// parseRmArgs validates the input arguments for the rm command
func parseRmArgs(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("exactly one argument required (e.g., cosm rm <package_name>)")
	}
	packageName := args[0]
	if packageName == "" {
		return "", fmt.Errorf("package name cannot be empty")
	}
	return packageName, nil
}

// findDependencyKey finds the keys for dependencies by package name
func findDependencyKey(project *types.Project, packageName string) ([]string, []types.Dependency, error) {
	var keys []string
	var deps []types.Dependency
	for key, dep := range project.Deps {
		if dep.Name == packageName {
			keys = append(keys, key)
			deps = append(deps, dep)
		}
	}
	if len(keys) == 0 {
		return nil, nil, fmt.Errorf("dependency '%s' not found in project", packageName)
	}
	return keys, deps, nil
}

// promptUserForDependency prompts the user to select a dependency when multiple have the same name
func promptUserForDependency(packageName string, keys []string, deps []types.Dependency) (string, error) {
	fmt.Printf("Multiple dependencies named '%s' found:\n", packageName)
	for i, dep := range deps {
		key := keys[i]
		parts := strings.Split(key, "@")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid key format for dependency '%s': %s", packageName, key)
		}
		fmt.Printf("  %d. Version %s (UUID: %s, Major Version: %s)\n", i+1, dep.Version, parts[0], parts[1])
	}
	fmt.Printf("Please select a dependency to remove (enter number 1-%d): ", len(deps))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())
	choiceNum := 0
	_, err := fmt.Sscanf(choice, "%d", &choiceNum)
	if err != nil || choiceNum < 1 || choiceNum > len(deps) {
		return "", fmt.Errorf("invalid selection '%s': must be a number between 1 and %d", choice, len(deps))
	}
	return keys[choiceNum-1], nil
}

// removeDependency deletes the dependency and saves the project
func removeDependency(project *types.Project, depKey, packageName string) error {
	delete(project.Deps, depKey)

	if err := saveProject(project, "Project.json"); err != nil {
		return err
	}

	fmt.Printf("Removed dependency '%s' from project\n", packageName)
	return nil
}
