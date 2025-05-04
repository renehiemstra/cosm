package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// Release updates the project version and publishes it to the remote repository and registries
func Release(cmd *cobra.Command, args []string) error {
	project, err := loadProject("Project.json")
	if err != nil {
		return err
	}
	if err := ensureNoUncommittedChanges(); err != nil {
		return err
	}
	if err := ensureLocalRepoInSyncWithOrigin(); err != nil {
		return err
	}
	newVersion, err := determineNewVersion(cmd, args, project.Version)
	if err != nil {
		return err
	}
	if err := validateNewVersion(newVersion, project.Version); err != nil {
		return err
	}
	if err := ensureTagDoesNotExist(newVersion); err != nil {
		return err
	}
	if err := updateProjectVersion(project, newVersion); err != nil {
		return err
	}
	if err := publishToGitRemote(newVersion); err != nil {
		return err
	}
	fmt.Printf("Released version '%s' for project '%s'\n", newVersion, project.Name)
	return nil
}

// determineNewVersion calculates the new version based on args or flags
func determineNewVersion(cmd *cobra.Command, args []string, currentVersion string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	if len(args) > 1 {
		return "", fmt.Errorf("too many arguments: use 'cosm release v<version>' or a version flag (--patch, --minor, --major)")
	}

	patch, _ := cmd.Flags().GetBool("patch")
	minor, _ := cmd.Flags().GetBool("minor")
	major, _ := cmd.Flags().GetBool("major")
	count := 0
	if patch {
		count++
	}
	if minor {
		count++
	}
	if major {
		count++
	}
	if count > 1 {
		return "", fmt.Errorf("only one of --patch, --minor, or --major can be specified")
	}
	if count == 0 {
		return "", fmt.Errorf("specify a version (e.g., v1.2.3) or use --patch, --minor, or --major")
	}

	currentSemVer, err := ParseSemVer(currentVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse current version: %v", err)
	}
	switch {
	case patch:
		return fmt.Sprintf("v%d.%d.%d", currentSemVer.Major, currentSemVer.Minor, currentSemVer.Patch+1), nil
	case minor:
		return fmt.Sprintf("v%d.%d.0", currentSemVer.Major, currentSemVer.Minor+1), nil
	case major:
		return fmt.Sprintf("v%d.0.0", currentSemVer.Major+1), nil
	}
	return "", fmt.Errorf("internal error: no version increment selected")
}

// validateNewVersion ensures the new version is valid and allowed
func validateNewVersion(newVersion, currentVersion string) error {
	// Parse versions
	currVer, err := ParseSemVer(currentVersion)
	if err != nil {
		return fmt.Errorf("invalid current version %q: %v", currentVersion, err)
	}
	newVer, err := ParseSemVer(newVersion)
	if err != nil {
		return fmt.Errorf("invalid new version %q: %v", newVersion, err)
	}

	// Allow same version if not tagged, otherwise require newer
	if newVersion == currentVersion {
		return nil // Tag existence checked later by ensureTagDoesNotExist
	}

	// Compare versions: newVer must be greater than currVer
	if newVer.Major < currVer.Major {
		return fmt.Errorf("new version %q must be greater than current version %q", newVersion, currentVersion)
	}
	if newVer.Major == currVer.Major {
		if newVer.Minor < currVer.Minor {
			return fmt.Errorf("new version %q must be greater than current version %q", newVersion, currentVersion)
		}
		if newVer.Minor == currVer.Minor && newVer.Patch <= currVer.Patch {
			return fmt.Errorf("new version %q must be greater than current version %q", newVersion, currentVersion)
		}
	}
	return nil
}

// ensureTagDoesNotExist checks if the new version tag already exists in the repo
func ensureTagDoesNotExist(newVersion string) error {
	tagsCmd := exec.Command("git", "tag")
	output, err := tagsCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list Git tags: %v", err)
	}
	tags := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, tag := range tags {
		if tag == newVersion {
			return fmt.Errorf("tag '%s' already exists in the repository", newVersion)
		}
	}
	return nil
}

// updateProjectVersion updates Project.json with the new version and commits the change
func updateProjectVersion(project *types.Project, newVersion string) error {
	if newVersion == project.Version {
		// No change needed, skip write and commit
		return nil
	}

	project.Version = newVersion
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Project.json: %v", err)
	}
	if err := os.WriteFile("Project.json", data, 0644); err != nil {
		return fmt.Errorf("failed to write Project.json: %v", err)
	}

	addCmd := exec.Command("git", "add", "Project.json")
	if addOutput, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage Project.json: %v\nOutput: %s", err, addOutput)
	}

	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("Release %s", newVersion))
	if commitOutput, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to commit release: %v\nOutput: %s", err, commitOutput)
	}

	return nil
}
