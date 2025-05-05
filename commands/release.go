package commands

import (
	"cosm/types"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// releaseConfig holds configuration for releasing a new project version
type releaseConfig struct {
	projectDir  string
	project     *types.Project
	newVersion  string
	patch       bool
	minor       bool
	major       bool
	projectFile string
}

// Release updates the project version and publishes it to the remote repository
func Release(cmd *cobra.Command, args []string) error {
	// Parse arguments and initialize config
	config, err := parseReleaseArgs(cmd, args)
	if err != nil {
		return err
	}

	// Validate repository state
	if err := validateRepositoryState(config); err != nil {
		return err
	}

	// Validate new version
	if err := validateReleaseVersion(config); err != nil {
		return err
	}

	// Update project version and commit
	if err := updateProjectVersion(config); err != nil {
		return err
	}

	// Publish to Git remote
	if err := publishToGitRemote(config); err != nil {
		return err
	}

	fmt.Printf("Released version '%s' for project '%s'\n", config.newVersion, config.project.Name)
	return nil
}

// parseReleaseArgs parses arguments and flags to initialize the release config
func parseReleaseArgs(cmd *cobra.Command, args []string) (*releaseConfig, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get project directory: %v", err)
	}
	projectFile := filepath.Join(projectDir, "Project.json")
	project, err := loadProject(projectFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %v", projectFile, err)
	}

	config := &releaseConfig{
		projectDir:  projectDir,
		project:     project,
		projectFile: projectFile,
	}

	if len(args) == 1 {
		config.newVersion = args[0]
		return config, nil
	}
	if len(args) > 1 {
		return nil, fmt.Errorf("too many arguments: use 'cosm release v<version>' or a version flag (--patch, --minor, --major)")
	}

	config.patch, _ = cmd.Flags().GetBool("patch")
	config.minor, _ = cmd.Flags().GetBool("minor")
	config.major, _ = cmd.Flags().GetBool("major")
	count := 0
	if config.patch {
		count++
	}
	if config.minor {
		count++
	}
	if config.major {
		count++
	}
	if count > 1 {
		return nil, fmt.Errorf("only one of --patch, --minor, or --major can be specified")
	}
	if count == 0 {
		return nil, fmt.Errorf("specify a version (e.g., v1.2.3) or use --patch, --minor, or --major")
	}

	currentSemVer, err := ParseSemVer(project.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current version '%s': %v", project.Version, err)
	}
	switch {
	case config.patch:
		config.newVersion = fmt.Sprintf("v%d.%d.%d", currentSemVer.Major, currentSemVer.Minor, currentSemVer.Patch+1)
	case config.minor:
		config.newVersion = fmt.Sprintf("v%d.%d.0", currentSemVer.Major, currentSemVer.Minor+1)
	case config.major:
		config.newVersion = fmt.Sprintf("v%d.0.0", currentSemVer.Major+1)
	}
	return config, nil
}

// validateRepositoryState ensures the repository is clean and in sync with origin
func validateRepositoryState(config *releaseConfig) error {
	if err := ensureNoUncommittedChanges(config.projectDir); err != nil {
		return fmt.Errorf("repository has uncommitted changes in %s: %v", config.projectDir, err)
	}
	if err := ensureLocalRepoInSyncWithOrigin(config.projectDir); err != nil {
		return fmt.Errorf("repository is not in sync with origin in %s: %v", config.projectDir, err)
	}
	return nil
}

// validateReleaseVersion validates the new version and ensures the tag doesn’t exist
func validateReleaseVersion(config *releaseConfig) error {
	if err := validateNewVersion(config.newVersion, config.project.Version); err != nil {
		return err
	}
	if err := ensureTagDoesNotExist(config.projectDir, config.newVersion); err != nil {
		return fmt.Errorf("failed to validate tag '%s' in %s: %v", config.newVersion, config.projectDir, err)
	}
	return nil
}

// updateProjectVersion updates Project.json with the new version and commits the change
func updateProjectVersion(config *releaseConfig) error {
	if config.newVersion == config.project.Version {
		// No change needed, skip write and commit
		return nil
	}

	config.project.Version = config.newVersion
	if err := saveProject(config.project, config.projectFile); err != nil {
		return fmt.Errorf("failed to save %s: %v", config.projectFile, err)
	}

	if err := stageFiles(config.projectDir, "Project.json"); err != nil {
		return fmt.Errorf("failed to stage %s in %s: %v", config.projectFile, config.projectDir, err)
	}

	commitMsg := fmt.Sprintf("Release %s", config.newVersion)
	if err := commitChanges(config.projectDir, commitMsg); err != nil {
		return fmt.Errorf("failed to commit release '%s' in %s: %v", config.newVersion, config.projectDir, err)
	}

	return nil
}

// publishToGitRemote tags and pushes the release to the remote repository
func publishToGitRemote(config *releaseConfig) error {
	// Tag the version
	if err := createTag(config.projectDir, config.newVersion); err != nil {
		return fmt.Errorf("failed to create tag '%s' in %s: %v", config.newVersion, config.projectDir, err)
	}

	// Get the current branch
	branch, err := getCurrentBranch(config.projectDir)
	if err != nil {
		return fmt.Errorf("failed to get current branch in %s: %v", config.projectDir, err)
	}

	// Push to the current branch
	if err := pushToRemote(config.projectDir, branch, true); err != nil {
		return err
	}

	// Push the tag
	return pushToRemote(config.projectDir, config.newVersion, false)
}

// ensureTagDoesNotExist checks if the new version tag already exists in the repo
func ensureTagDoesNotExist(projectDir, newVersion string) error {
	tags, err := listTags(projectDir)
	if err != nil {
		return fmt.Errorf("failed to list tags in %s: %v", projectDir, err)
	}
	for _, tag := range tags {
		if tag == newVersion {
			return fmt.Errorf("tag '%s' already exists in the repository", newVersion)
		}
	}
	return nil
}
