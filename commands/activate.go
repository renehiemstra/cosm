package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Activate computes the build list for the current project under development
func Activate(cmd *cobra.Command, args []string) error {
	project, projectStat, err := validateActivate(args)
	if err != nil {
		return err
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return fmt.Errorf("failed to get cosm directory: %v", err)
	}
	registriesDir := setupRegistriesDir(cosmDir)
	buildListFile := ".cosm/buildlist.json"

	if err := generateOrVerifyBuildList(project, projectStat, registriesDir, buildListFile); err != nil {
		return err
	}

	// Load build list
	buildList, err := loadBuildListFile(buildListFile)
	if err != nil {
		return fmt.Errorf("failed to load buildlist.json: %v", err)
	}

	// Generate environment variables
	if err := generateEnvironmentVariables(cosmDir, &buildList); err != nil {
		return fmt.Errorf("failed to generate environment variables: %v", err)
	}

	// Make all packages available
	if err := makePackagesAvailable(&buildList, cosmDir); err != nil {
		return fmt.Errorf("failed to make packages available: %v", err)
	}

	// Start a new interactive shell
	if err := startInteractiveShell(); err != nil {
		return err
	}

	return nil
}

// validateActivate checks if the command is run in a valid package root with no arguments
func validateActivate(args []string) (*types.Project, os.FileInfo, error) {
	if len(args) != 0 {
		return nil, nil, fmt.Errorf("cosm activate takes no arguments; run in package root with Project.json")
	}
	projectFile := "Project.json"
	projectStat, err := os.Stat(projectFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("Project.json not found in current directory")
		}
		return nil, nil, fmt.Errorf("failed to stat Project.json: %v", err)
	}
	project, err := loadProject(projectFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse Project.json: %v", err)
	}
	return project, projectStat, nil
}

// generateOrVerifyBuildList generates the build list if needed or verifies it’s up-to-date
func generateOrVerifyBuildList(project *types.Project, projectStat os.FileInfo, registriesDir, buildListFile string) error {
	needsBuildList, err := needsBuildListGeneration(projectStat)
	if err != nil {
		return err
	}

	if needsBuildList {
		if err := createEnvironmentFiles(); err != nil {
			return err
		}
		if err := generateLocalBuildList(project, registriesDir); err != nil {
			return err
		}
		fmt.Printf("Generated build list for %s in %s\n", project.Name, buildListFile)
	} else {
		fmt.Printf("Build list up-to-date in %s\n", buildListFile)
	}
	return nil
}

// needsBuildListGeneration checks if buildlist.json needs regeneration based on mod times
func needsBuildListGeneration(projectStat os.FileInfo) (bool, error) {
	buildListFile := ".cosm/buildlist.json"
	buildListStat, err := os.Stat(buildListFile)
	if err == nil {
		return !buildListStat.ModTime().After(projectStat.ModTime()), nil
	}
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, fmt.Errorf("failed to stat %s: %v", buildListFile, err)
}

// generateLocalBuildList computes and writes the build list to .cosm/buildlist.json
func generateLocalBuildList(project *types.Project, registriesDir string) error {
	buildList, err := generateBuildList(project, registriesDir)
	if err != nil {
		return fmt.Errorf("failed to generate build list for %s: %v", project.Name, err)
	}
	data, err := json.MarshalIndent(buildList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal buildlist.json: %v", err)
	}
	buildListFile := ".cosm/buildlist.json"
	if err := os.WriteFile(buildListFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %v", buildListFile, err)
	}
	return nil
}

// createEnvironmentFiles creates .cosm directory, .env, and .bashrc
func createEnvironmentFiles() error {
	if err := os.MkdirAll(".cosm", 0755); err != nil {
		return fmt.Errorf("failed to create .cosm directory: %v", err)
	}
	const bashrcContent = `# signal that cosm prompt is active
		export COSM_PROMPT=1

		# supress depracation warning
		export BASH_SILENCE_DEPRECATION_WARNING=1

		# define cosm prompt
		function customp {
			BOLD="\[$(tput bold)\]"
			NORMAL="\[$(tput sgr0)\]"
			GREEN="\[$(tput setaf 2)\]"
			WHITE="\[$(tput setaf 7)\]"
			PROMPT="\[cosm>\]"
			PS1="$BOLD$GREEN$PROMPT$NORMAL$WHITE "
		}
		customp

		# reload environment variables in every command
		function before_command() {
		case "$BASH_COMMAND" in
			$PROMPT_COMMAND)
			;;
			*)
			if [ -f .cosm/.env ]; then
				source .cosm/.env
			fi
			;;
		esac
		}
		trap before_command DEBUG
		`
	if err := os.WriteFile(".cosm/.bashrc", []byte(bashrcContent), 0644); err != nil {
		return fmt.Errorf("failed to write .cosm/.bashrc: %v", err)
	}
	return nil
}

// generateEnvironmentVariables creates the .cosm/.env file with environment variables
func generateEnvironmentVariables(cosmDir string, buildList *types.BuildList) error {
	// Construct TERRA_PATH
	var terraPaths []string
	terraPaths = append(terraPaths, "src/?.t")
	for _, dep := range buildList.Dependencies {
		if dep.Path != "" {
			pathVar := filepath.Join(cosmDir, dep.Path, "src", "?.t")
			terraPaths = append(terraPaths, pathVar)
		}
	}
	terraPathValue := strings.Join(terraPaths, ";") + ";;"

	// Write to .cosm/.env
	envContent := fmt.Sprintf("export TERRA_PATH=%q\n", terraPathValue)
	envFile := filepath.Join(".", ".cosm", ".env")
	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		return fmt.Errorf("failed to write .cosm/.env: %v", err)
	}

	return nil
}

// makePackagesAvailable ensures all packages in the build list are available
func makePackagesAvailable(buildList *types.BuildList, cosmDir string) error {
	registriesDir := setupRegistriesDir(cosmDir)
	// Process all dependencies
	for _, dep := range buildList.Dependencies {
		specs, _, err := findDependency(dep.Name, dep.Version, dep.UUID, registriesDir)
		if err != nil {
			return err
		}
		if err := MakePackageAvailable(cosmDir, &specs); err != nil {
			return fmt.Errorf("failed to make package '%s@%s' available: %v", dep.Name, dep.Version, err)
		}
	}
	return nil
}

// startBashShell starts a new bash shell sourcing .cosm/.bashrc
func startInteractiveShell() error {
	bashrcFile := filepath.Join(".cosm", ".bashrc")
	cmdShell := exec.Command("bash", "--rcfile", bashrcFile)
	cmdShell.Stdin = os.Stdin
	cmdShell.Stdout = os.Stdout
	cmdShell.Stderr = os.Stderr
	fmt.Printf("Starting interactive shell. Press ctrl-d or type 'exit' to quit.\n")
	if err := cmdShell.Run(); err != nil {
		return fmt.Errorf("failed to start bash shell with .cosm/.bashrc: %v", err)
	}
	return nil
}
