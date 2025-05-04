package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// getGitAuthors retrieves the author info from git config or uses a default
func getGitAuthors() ([]string, error) {
	name, errName := exec.Command("git", "config", "user.name").Output()
	email, errEmail := exec.Command("git", "config", "user.email").Output()
	if errName != nil || errEmail != nil || len(name) == 0 || len(email) == 0 {
		fmt.Println("Warning: Could not retrieve git user.name or user.email, defaulting to '[unknown]unknown@author.com'")
		return []string{"[unknown]unknown@author.com"}, nil // Return default with no error
	}
	gitName := strings.TrimSpace(string(name))
	gitEmail := strings.TrimSpace(string(email))
	return []string{fmt.Sprintf("[%s]%s", gitName, gitEmail)}, nil
}

// revertClone returns the clone to its previous branch or state using 'git checkout -'
func revertClone(clonePath string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(clonePath); err != nil {
		cleanupRevert(currentDir)
		return fmt.Errorf("failed to change to clone directory %s: %v", clonePath, err)
	}

	// Revert to the previous branch or commit state
	cmd := exec.Command("git", "checkout", "-")
	if output, err := cmd.CombinedOutput(); err != nil {
		cleanupRevert(currentDir)
		return fmt.Errorf("failed to revert clone to previous state: %v\nOutput: %s", err, output)
	}

	if err := restoreRevertDir(currentDir); err != nil {
		return err
	}
	return nil
}

// cleanupRevert reverts to the original directory
func cleanupRevert(originalDir string) {
	if err := os.Chdir(originalDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to return to original directory %s: %v\n", originalDir, err)
	}
}

// restoreRevertDir returns to the original directory
func restoreRevertDir(originalDir string) error {
	if err := os.Chdir(originalDir); err != nil {
		return fmt.Errorf("failed to return to original directory %s: %v", originalDir, err)
	}
	return nil
}

// commitAndPushRegistryChanges stages, commits, and pushes changes to the registry
func commitAndPushRegistryChanges(registriesDir, registryName, commitMsg string) error {
	registryDir := filepath.Join(registriesDir, registryName)
	if err := os.Chdir(registryDir); err != nil {
		return fmt.Errorf("failed to change to registry directory %s: %v", registryName, err)
	}
	addCmd := exec.Command("git", "add", ".")
	if addOutput, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage registry changes: %v\nOutput: %s", err, addOutput)
	}
	commitCmd := exec.Command("git", "commit", "-m", commitMsg)
	if commitOutput, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to commit registry changes: %v\nOutput: %s", err, commitOutput)
	}
	pushCmd := exec.Command("git", "push", "origin", "main")
	if pushOutput, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push registry changes: %v\nOutput: %s", err, pushOutput)
	}
	return nil
}

// checkoutVersion switches the clone to the specified SHA1
func checkoutVersion(clonePath, sha1 string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(clonePath); err != nil {
		cleanupCheckout(currentDir)
		return fmt.Errorf("failed to change to clone directory %s: %v", clonePath, err)
	}

	// Fetch updates to ensure we have the latest refs
	if err := exec.Command("git", "fetch", "origin").Run(); err != nil {
		cleanupCheckout(currentDir)
		return fmt.Errorf("failed to fetch updates: %v", err)
	}

	// Checkout the specific SHA1
	cmd := exec.Command("git", "checkout", sha1)
	if output, err := cmd.CombinedOutput(); err != nil {
		cleanupCheckout(currentDir)
		return fmt.Errorf("failed to checkout SHA1 %s: %v\nOutput: %s", sha1, err, output)
	}

	if err := restoreCheckoutDir(currentDir); err != nil {
		return err
	}
	return nil
}

// cleanupCheckout reverts to the original directory
func cleanupCheckout(originalDir string) {
	if err := os.Chdir(originalDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to return to original directory %s: %v\n", originalDir, err)
	}
}

// restoreCheckoutDir returns to the original directory
func restoreCheckoutDir(originalDir string) error {
	if err := os.Chdir(originalDir); err != nil {
		return fmt.Errorf("failed to return to original directory %s: %v", originalDir, err)
	}
	return nil
}

// publishToGitRemote tags and pushes the release to the remote repository
func publishToGitRemote(version string) error {
	tagCmd := exec.Command("git", "tag", version)
	if tagOutput, err := tagCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to tag version %q: %v\nOutput: %s", version, err, tagOutput)
	}

	pushCmd := exec.Command("git", "push", "origin", "main")
	if pushOutput, err := pushCmd.CombinedOutput(); err != nil {
		// Ignore "everything up-to-date" errors
		if !strings.Contains(string(pushOutput), "Everything up-to-date") {
			return fmt.Errorf("failed to push to origin main: %v\nOutput: %s", err, pushOutput)
		}
	}

	pushTagCmd := exec.Command("git", "push", "origin", version)
	if pushTagOutput, err := pushTagCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push tag %q: %v\nOutput: %s", version, err, pushTagOutput)
	}

	return nil
}

// ensureNoUncommittedChanges checks for uncommitted changes in the Git repo
func ensureNoUncommittedChanges() error {
	statusCmd := exec.Command("git", "status", "--porcelain")
	output, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check Git status: %v", err)
	}
	if len(strings.TrimSpace(string(output))) > 0 {
		return fmt.Errorf("repository has uncommitted changes: please commit or stash them before releasing")
	}
	return nil
}

// ensureLocalRepoInSyncWithOrigin ensures the local repo is ahead or in sync with origin
func ensureLocalRepoInSyncWithOrigin() error {
	fetchCmd := exec.Command("git", "fetch", "origin")
	if err := fetchCmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch from origin: %v", err)
	}
	// Check if local is behind origin
	revListCmd := exec.Command("git", "rev-list", "--count", "HEAD..origin/main")
	output, err := revListCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check sync with origin: %v", err)
	}
	behindCount, _ := strconv.Atoi(strings.TrimSpace(string(output)))
	if behindCount > 0 {
		return fmt.Errorf("local repository is behind origin: please pull changes before proceeding")
	}
	return nil
}
