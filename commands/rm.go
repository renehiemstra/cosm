package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Rm removes a dependency from the project's Project.json file
func Rm(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one argument required (e.g., cosm rm <package_name>)")
	}
	packageName := args[0]
	if packageName == "" {
		return fmt.Errorf("package name cannot be empty")
	}

	project, err := loadProject("Project.json")
	if err != nil {
		return err
	}

	if _, exists := project.Deps[packageName]; !exists {
		return fmt.Errorf("dependency '%s' not found in project", packageName)
	}

	delete(project.Deps, packageName)

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Project.json: %v", err)
	}
	if err := os.WriteFile("Project.json", data, 0644); err != nil {
		return fmt.Errorf("failed to write Project.json: %v", err)
	}

	fmt.Printf("Removed dependency '%s' from project\n", packageName)
	return nil
}
