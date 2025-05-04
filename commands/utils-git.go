package commands

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// getCurrentBranch retrieves the current branch name for the repository in the specified directory.
func getCurrentBranch(dir string) (string, error) {
	branch, err := runCommand(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
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
	output, err := runCommand(dir, "git", "push", "origin", target)
	if err != nil && !(ignoreUpToDate && strings.Contains(output, "Everything up-to-date")) {
		return fmt.Errorf("failed to push %s to origin in %s: %v", target, dir, err)
	}
	return nil
}

// fetchOrigin fetches updates from origin.
func fetchOrigin(dir string) error {
	if _, err := runCommand(dir, "git", "fetch", "origin"); err != nil {
		return wrapGitError(dir, "failed to fetch from origin", err)
	}
	return nil
}

// getGitAuthors retrieves the author info from git config or uses a default
func getGitAuthors() ([]string, error) {
	name, errName := runCommand("", "git", "config", "user.name")
	if errName != nil {
		name = ""
	}
	email, errEmail := runCommand("", "git", "config", "user.email")
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
	_, err := runCommand(clonePath, "git", "checkout", "-")
	return err
}

// commitAndPushRegistryChanges stages, commits, and pushes changes to the registry
func commitAndPushRegistryChanges(registriesDir, registryName, commitMsg string) error {
	registryDir := filepath.Join(registriesDir, registryName)

	// Stage changes
	if _, err := runCommand(registryDir, "git", "add", "."); err != nil {
		return wrapGitError(registryDir, "failed to stage registry changes", err)
	}

	// Commit changes
	if _, err := runCommand(registryDir, "git", "commit", "-m", commitMsg); err != nil {
		return wrapGitError(registryDir, "failed to commit registry changes", err)
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
	if _, err := runCommand(clonePath, "git", "checkout", sha1); err != nil {
		return fmt.Errorf("failed to checkout SHA1 %s in %s: %v", sha1, clonePath, err)
	}

	return nil
}

// publishToGitRemote tags and pushes the release to the remote repository
func publishToGitRemote(projectDir, version string) error {
	// Tag the version
	if _, err := runCommand(projectDir, "git", "tag", version); err != nil {
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
	output, err := runCommand(projectDir, "git", "status", "--porcelain")
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
	output, err := runCommand(projectDir, "git", "rev-list", "--count", fmt.Sprintf("HEAD..origin/%s", branch))
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
