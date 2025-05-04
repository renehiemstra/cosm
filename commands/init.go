package commands

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// Init initializes a new project with a Project.json file
func Init(cmd *cobra.Command, args []string) error {
	packageName, version, err := validateInitArgs(args, cmd)
	if err != nil {
		return err
	}
	language := getInitLanguageFlag(cmd)
	if version != "" {
		if err := validateVersion(version); err != nil {
			return err
		}
	}
	projectUUID := uuid.New().String()
	authors, err := getGitAuthors()
	if err != nil {
		return err
	}
	if err := ensureProjectFileDoesNotExist("Project.json"); err != nil {
		return err
	}
	project := createProject(packageName, projectUUID, authors, language, version)
	data, err := marshalProject(project)
	if err != nil {
		return err
	}
	if err := writeProjectFile("Project.json", data); err != nil {
		return err
	}
	fmt.Printf("Initialized project '%s' with version %s\n", packageName, version)
	return nil
}

// validateInitArgs checks the command-line arguments for validity
func validateInitArgs(args []string, cmd *cobra.Command) (string, string, error) {
	if len(args) < 1 || len(args) > 2 {
		return "", "", fmt.Errorf("one or two arguments required (e.g., cosm init <package-name> [version])")
	}
	packageName := args[0]
	if packageName == "" {
		return "", "", fmt.Errorf("package name cannot be empty")
	}

	// Check version from args or flag
	version := ""
	if len(args) == 2 {
		version = args[1]
	}
	flagVersion, _ := cmd.Flags().GetString("version")
	if version != "" && flagVersion != "" {
		return "", "", fmt.Errorf("cannot specify version both as an argument and a flag")
	}
	if version == "" {
		version = flagVersion
	}
	if version == "" {
		version = "v0.1.0" // Default version
	}
	return packageName, version, nil
}

// getInitLanguageFlag retrieves the language flag from the command
func getInitLanguageFlag(cmd *cobra.Command) string {
	language, _ := cmd.Flags().GetString("language")
	return language
}
