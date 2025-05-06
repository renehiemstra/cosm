package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// Init initializes a new project with a Project.json file
func Init(cmd *cobra.Command, args []string) error {
	templatePath, _ := cmd.Flags().GetString("template")
	if templatePath != "" {
		return initWithTemplate(cmd, args, templatePath)
	}
	return initWithoutTemplate(cmd, args)
}

// Init initializes a new project with a Project.json file
func initWithoutTemplate(cmd *cobra.Command, args []string) error {
	packageName, version, err := validateInitArgsWithoutTemplate(args, cmd)
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
	if err := saveProject(&project, "Project.json"); err != nil {
		return err
	}
	fmt.Printf("Initialized project '%s' with version %s\n", packageName, version)
	return nil
}

// validateInitArgs checks the command-line arguments for validity
func validateInitArgsWithoutTemplate(args []string, cmd *cobra.Command) (string, string, error) {
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

// initWithTemplate initializes a project using a template
func initWithTemplate(cmd *cobra.Command, args []string, templatePath string) error {
	packageName, version, err := validateInitArgsWithTemplate(args, cmd)
	if err != nil {
		return err
	}

	// Determine language from template path
	parts := strings.Split(templatePath, string(filepath.Separator))
	if len(parts) < 2 {
		return fmt.Errorf("template path %s must start with <language>/", templatePath)
	}
	language := parts[0]

	// Create project directory
	projectDir := packageName
	if err := os.Mkdir(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory %s: %v", projectDir, err)
	}

	// Copy template files
	templateName := filepath.Base(templatePath)
	if err := copyTemplateFiles(templatePath, projectDir, templateName, packageName); err != nil {
		return fmt.Errorf("failed to copy template files: %v", err)
	}

	// Initialize project
	projectUUID := uuid.New().String()
	authors, err := getGitAuthors()
	if err != nil {
		return err
	}
	projectFile := filepath.Join(projectDir, "Project.json")
	if err := ensureProjectFileDoesNotExist(projectFile); err != nil {
		return err
	}
	project := createProject(packageName, projectUUID, authors, language, version)
	if err := saveProject(&project, projectFile); err != nil {
		return err
	}

	// Initialize git repository
	if err := initializeGitRepo(projectDir); err != nil {
		return fmt.Errorf("failed to initialize git repository: %v", err)
	}

	fmt.Printf("Initialized project '%s' with version %s in %s\n", packageName, version, projectDir)
	return nil
}

// validateInitArgsWithTemplate checks the command-line arguments and flags for template mode
func validateInitArgsWithTemplate(args []string, cmd *cobra.Command) (string, string, error) {
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
	if version != "" {
		if err := validateVersion(version); err != nil {
			return "", "", err
		}
	}

	// Disallow --language with --template
	if language, _ := cmd.Flags().GetString("language"); language != "" {
		return "", "", fmt.Errorf("cannot specify --language when using --template")
	}

	// Validate template path
	templatePath, _ := cmd.Flags().GetString("template")
	cosmDir, err := getCosmDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get cosm directory: %v", err)
	}
	templateFullPath := filepath.Join(cosmDir, "templates", templatePath)
	if _, err := os.Stat(templateFullPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("template directory %s does not exist", templateFullPath)
	}
	// Validate template path starts with <language>/
	parts := strings.Split(templatePath, string(filepath.Separator))
	if len(parts) < 2 {
		return "", "", fmt.Errorf("template path %s must start with <language>/", templatePath)
	}

	return packageName, version, nil
}

// copyTemplateFiles copies files from the template directory to the project directory, replacing templateName with packageName in contents and filenames
func copyTemplateFiles(templatePath, projectDir, templateName, packageName string) error {
	cosmDir, err := getCosmDir()
	if err != nil {
		return fmt.Errorf("failed to get cosm directory: %v", err)
	}
	templateFullPath := filepath.Join(cosmDir, "templates", templatePath)

	return filepath.Walk(templateFullPath, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Compute relative path and destination
		relPath, err := filepath.Rel(templateFullPath, srcPath)
		if err != nil {
			return fmt.Errorf("failed to compute relative path for %s: %v", srcPath, err)
		}
		if relPath == "." {
			return nil // Skip root directory itself
		}

		// Determine destination filename, renaming <templateName>.* to <packageName>.*
		destRelPath := relPath
		baseName := filepath.Base(relPath)
		ext := filepath.Ext(baseName)
		nameWithoutExt := strings.TrimSuffix(baseName, ext)
		if nameWithoutExt == templateName {
			newBaseName := packageName + ext
			destRelPath = filepath.Join(filepath.Dir(relPath), newBaseName)
		}
		destPath := filepath.Join(projectDir, destRelPath)

		// Handle directories
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Copy and replace content for text files
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %v", srcPath, err)
		}
		// Assume text files for simplicity; skip binary files if needed
		content := string(data)
		newContent := strings.ReplaceAll(content, templateName, packageName)

		if err := os.WriteFile(destPath, []byte(newContent), info.Mode()); err != nil {
			return fmt.Errorf("failed to write file %s: %v", destPath, err)
		}

		return nil
	})
}

// initializeGitRepo initializes a git repository, adds all files, and commits
func initializeGitRepo(projectDir string) error {
	// Run git init
	if _, err := gitCommand(projectDir, "init"); err != nil {
		return fmt.Errorf("failed to initialize git repository in %s: %v", projectDir, err)
	}

	// Add all files
	if err := stageFiles(projectDir, "."); err != nil {
		return fmt.Errorf("failed to stage files in %s: %v", projectDir, err)
	}

	// Commit files
	if err := commitChanges(projectDir, "Initial commit"); err != nil {
		return fmt.Errorf("failed to commit files in %s: %v", projectDir, err)
	}

	return nil
}

// getInitLanguageFlag retrieves the language flag from the command
func getInitLanguageFlag(cmd *cobra.Command) string {
	language, _ := cmd.Flags().GetString("language")
	return language
}
