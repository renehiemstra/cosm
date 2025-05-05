package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Activate computes the build list for the current project under development
func Activate(cmd *cobra.Command, args []string) error {
	project, projectStat, err := validateActivate(args)
	if err != nil {
		return err
	}

	needsBuildList, err := needsBuildListGeneration(projectStat)
	if err != nil {
		return err
	}

	if !needsBuildList {
		fmt.Printf("Build list up-to-date in .cosm/buildlist.json\n")
		return nil
	}

	if err := createEnvironmentFiles(); err != nil {
		return err
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return fmt.Errorf("failed to get cosm directory: %v", err)
	}
	registriesDir := setupRegistriesDir(cosmDir)

	if err := generateLocalBuildList(project, registriesDir); err != nil {
		return err
	}

	fmt.Printf("Generated build list for %s in .cosm/buildlist.json\n", project.Name)
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
	if err := os.WriteFile(".cosm/.env", []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to write .cosm/.env: %v", err)
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
