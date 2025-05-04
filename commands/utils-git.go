package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// getCurrentBranch retrieves the current branch name for the repository in the specified directory.
func getCurrentBranch(dir string) (string, error) {
	branch, err := gitCommand(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to determine current branch in %s: %v", dir, err)
	}
	if branch == "" {
		return "", fmt.Errorf("no current branch detected in %s", dir)
	}
	return branch, nil
}

// wrapGitError wraps a Git command error with directory context.
func wrapGitError(dir, msg string, err error) error {
	return fmt.Errorf("%s in %s: %v", msg, dir, err)
}

// pushToRemote pushes the specified target (branch or tag) to origin.
func pushToRemote(dir, target string, ignoreUpToDate bool) error {
	output, err := gitCommand(dir, "push", "origin", target)
	if err != nil && !(ignoreUpToDate && strings.Contains(output, "Everything up-to-date")) {
		return fmt.Errorf("failed to push %s to origin in %s: %v", target, dir, err)
	}
	return nil
}

// fetchOrigin fetches updates from origin.
func fetchOrigin(dir string) error {
	if _, err := gitCommand(dir, "fetch", "origin"); err != nil {
		return wrapGitError(dir, "failed to fetch from origin", err)
	}
	return nil
}

// gitCommand executes a Git command in the specified directory, returning the output and any error.
// The subcommand is the Git command (e.g., "add", "commit"), followed by its arguments.
func gitCommand(dir, subcommand string, args ...string) (string, error) {
	if subcommand == "" {
		return "", fmt.Errorf("no Git subcommand provided for directory %s", dir)
	}
	cmdArgs := append([]string{"git", subcommand}, args...)
	output, err := runCommand(dir, cmdArgs...)
	if err != nil && strings.Contains(output, "nothing to commit") && subcommand == "commit" {
		return output, nil // Ignore "nothing to commit" errors for git commit
	}
	return output, err
}

// getGitAuthors retrieves the author info from git config or uses a default
func getGitAuthors() ([]string, error) {
	// Use empty directory for global/system-wide config
	name, errName := gitCommand("", "config", "user.name")
	if errName != nil {
		name = ""
	}
	email, errEmail := gitCommand("", "config", "user.email")
	if errEmail != nil {
		email = ""
	}
	if name == "" || email == "" {
		fmt.Println("Warning: Could not retrieve git user.name or user.email, defaulting to '[unknown]unknown@author.com'")
		return []string{"[unknown]unknown@author.com"}, nil
	}
	return []string{fmt.Sprintf("[%s]%s", name, email)}, nil
}

// revertClone returns the clone to its previous branch or state using 'git checkout -'
func revertClone(clonePath string) error {
	_, err := gitCommand(clonePath, "checkout", "-")
	return err
}

// stageFiles stages the specified files or directories using git add.
func stageFiles(dir string, paths ...string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths provided to stage in %s", dir)
	}
	_, err := gitCommand(dir, "add", paths...)
	if err != nil {
		return wrapGitError(dir, "failed to stage changes", err)
	}
	return nil
}

// commitChanges commits staged changes with the specified message.
func commitChanges(dir, message string) error {
	_, err := gitCommand(dir, "commit", "-m", message)
	if err != nil {
		return wrapGitError(dir, "failed to commit changes", err)
	}
	return nil
}

// clone clones a repository from gitURL to the destination directory.
func clone(gitURL, destination string) (string, error) {
	if _, err := gitCommand(filepath.Dir(destination), "clone", gitURL, destination); err != nil {
		return "", fmt.Errorf("failed to clone repository from '%s' to %s: %v", gitURL, destination, err)
	}
	return destination, nil
}

// commitAndPushRegistryChanges stages, commits, and pushes changes to the registry
func commitAndPushRegistryChanges(registriesDir, registryName, commitMsg string) error {
	registryDir := filepath.Join(registriesDir, registryName)

	// Stage all changes
	if err := stageFiles(registryDir, "."); err != nil {
		return err
	}

	// Commit changes
	if err := commitChanges(registryDir, commitMsg); err != nil {
		return err
	}

	// Get the current branch
	branch, err := getCurrentBranch(registryDir)
	if err != nil {
		return err
	}

	// Push changes to the current branch
	return pushToRemote(registryDir, branch, false)
}

// checkoutVersion switches the clone to the specified SHA1
func checkoutVersion(clonePath, sha1 string) error {
	// Fetch updates to ensure we have the latest refs
	if err := fetchOrigin(clonePath); err != nil {
		return err
	}

	// Checkout the specific SHA1
	_, err := gitCommand(clonePath, "checkout", sha1)
	if err != nil {
		return fmt.Errorf("failed to checkout SHA1 %s in %s: %v", sha1, clonePath, err)
	}
	return nil
}

// publishToGitRemote tags and pushes the release to the remote repository
func publishToGitRemote(projectDir, version string) error {
	// Tag the version
	_, err := gitCommand(projectDir, "tag", version)
	if err != nil {
		return wrapGitError(projectDir, fmt.Sprintf("failed to tag version %q", version), err)
	}

	// Get the current branch
	branch, err := getCurrentBranch(projectDir)
	if err != nil {
		return err
	}

	// Push to the current branch
	if err := pushToRemote(projectDir, branch, true); err != nil {
		return err
	}

	// Push the tag
	return pushToRemote(projectDir, version, false)
}

// ensureNoUncommittedChanges checks for uncommitted changes in the Git repo
func ensureNoUncommittedChanges(projectDir string) error {
	output, err := gitCommand(projectDir, "status", "--porcelain")
	if err != nil {
		return wrapGitError(projectDir, "failed to check Git status", err)
	}
	if len(strings.TrimSpace(output)) > 0 {
		return fmt.Errorf("repository has uncommitted changes in %s: please commit or stash them before releasing", projectDir)
	}
	return nil
}

// ensureLocalRepoInSyncWithOrigin ensures the local repo is ahead or in sync with origin
func ensureLocalRepoInSyncWithOrigin(projectDir string) error {
	// Get the current branch
	branch, err := getCurrentBranch(projectDir)
	if err != nil {
		return err
	}

	// Fetch updates from origin
	if err := fetchOrigin(projectDir); err != nil {
		return err
	}

	// Check if local is behind origin
	output, err := gitCommand(projectDir, "rev-list", "--count", fmt.Sprintf("HEAD..origin/%s", branch))
	if err != nil {
		return fmt.Errorf("failed to check sync with origin/%s in %s: %v", branch, projectDir, err)
	}
	behindCount, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil {
		return wrapGitError(projectDir, "failed to parse behind count", err)
	}
	if behindCount > 0 {
		return fmt.Errorf("local repository is behind origin/%s in %s: please pull changes before proceeding", branch, projectDir)
	}

	return nil
}

// commitAndPushInitialRegistryChanges stages, commits, and pushes the initial registry changes
func commitAndPushInitialRegistryChanges(registryName string) error {
	registriesDir, err := getRegistriesDir()
	if err != nil {
		return err
	}
	registryDir := filepath.Join(registriesDir, registryName)

	// Stage registry.json
	if err := stageFiles(registryDir, "registry.json"); err != nil {
		return err
	}

	// Commit changes
	commitMsg := fmt.Sprintf("Initialized registry %s", registryName)
	if err := commitChanges(registryDir, commitMsg); err != nil {
		return err
	}

	// Get the current branch
	branch, err := getCurrentBranch(registryDir)
	if err != nil {
		return err
	}

	// Push changes to the current branch
	return pushToRemote(registryDir, branch, false)
}

// clonePackageToTempDir creates a temp clone directly in the clones directory
func clonePackageToTempDir(cosmDir, packageGitURL string) (string, error) {
	clonesDir := filepath.Join(cosmDir, "clones")
	if err := os.MkdirAll(clonesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create clones directory: %v", err)
	}
	tmpClonePath := filepath.Join(clonesDir, "tmp-clone")
	if _, err := clone(packageGitURL, tmpClonePath); err != nil {
		cleanupErr := cleanupTempClone(tmpClonePath)
		if cleanupErr != nil {
			return "", fmt.Errorf("failed to clone package repository at '%s': %v; cleanup failed: %v", packageGitURL, err, cleanupErr)
		}
		return "", fmt.Errorf("failed to clone package repository at '%s': %v", packageGitURL, err)
	}
	return tmpClonePath, nil
}
