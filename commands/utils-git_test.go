package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestEnv sets up a temporary environment with a Git config
func setupTestEnv(t *testing.T) (tempDir string, cleanup func()) {
	t.Helper()
	tempDir = t.TempDir()
	os.Setenv("HOME", tempDir)
	_ = setupTempGitConfig(t, tempDir)
	cleanup = func() { os.Unsetenv("HOME") }
	return tempDir, cleanup
}

// setupTempGitConfig creates a temporary Git config file and sets mock values
func setupTempGitConfig(t *testing.T, tempDir string) string {
	t.Helper()
	tempGitConfig := filepath.Join(tempDir, "gitconfig")

	// Write a complete Git config file directly
	configContent := `[user]
	name = testuser
	email = testuser@git.com
`
	if err := os.WriteFile(tempGitConfig, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create temporary Git config file %s: %v", tempGitConfig, err)
	}

	// Set GIT_CONFIG_GLOBAL to point to this file
	os.Setenv("GIT_CONFIG_GLOBAL", tempGitConfig)
	t.Cleanup(func() { os.Unsetenv("GIT_CONFIG_GLOBAL") }) // Clean up after test

	// Verify Git recognizes the config (for debugging, non-fatal)
	cmd := exec.Command("git", "config", "--global", "--get", "user.name")
	cmd.Dir = tempDir // Ensure a stable working directory
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Debug: Git config verification failed: %v\nOutput: %s", err, output)
	} else if strings.TrimSpace(string(output)) != "testuser" {
		t.Logf("Debug: Expected git user.name 'testuser', got %q", strings.TrimSpace(string(output)))
	}

	return tempGitConfig
}

// TestClone_Success tests the clone function by cloning a real Git repository.
func TestClone_Success(t *testing.T) {
	// Setup temporary environment with Git config
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create directories for local and bare repositories
	localDir := filepath.Join(tempDir, "local")
	bareDir := filepath.Join(tempDir, "bare.git")

	// Create localDir
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("Failed to create local directory %s: %v", localDir, err)
	}

	// Initialize local repository
	if _, err := GitCommand(localDir, "init"); err != nil {
		t.Fatalf("Failed to init local Git repo in %s: %v", localDir, err)
	}

	// Create and commit Project.json
	projectFile := filepath.Join(localDir, "Project.json")
	if err := os.WriteFile(projectFile, []byte(`{"name": "test", "uuid": "1234"}`), 0644); err != nil {
		t.Fatalf("Failed to create Project.json in %s: %v", localDir, err)
	}
	if _, err := GitCommand(localDir, "add", "Project.json"); err != nil {
		t.Fatalf("Failed to add Project.json in %s: %v", localDir, err)
	}
	if _, err := GitCommand(localDir, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("Failed to commit in %s: %v", localDir, err)
	}
	if _, err := GitCommand(localDir, "branch", "-m", "main"); err != nil {
		t.Fatalf("Failed to set main branch in %s: %v", localDir, err)
	}

	// Create bareDir
	if err := os.MkdirAll(bareDir, 0755); err != nil {
		t.Fatalf("Failed to create bare directory %s: %v", bareDir, err)
	}

	// Initialize bare repository and set HEAD
	if _, err := GitCommand(bareDir, "init", "--bare"); err != nil {
		t.Fatalf("Failed to init bare Git repo in %s: %v", bareDir, err)
	}
	if _, err := GitCommand(bareDir, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
		t.Fatalf("Failed to set HEAD in bare repo %s: %v", bareDir, err)
	}

	// Add bare repository as remote and push
	if _, err := GitCommand(localDir, "remote", "add", "origin", bareDir); err != nil {
		t.Fatalf("Failed to add remote in %s: %v", localDir, err)
	}
	if output, err := GitCommand(localDir, "push", "origin", "main"); err != nil {
		t.Fatalf("Failed to push to bare repo from %s: %v\nOutput: %s", localDir, err, output)
	}

	// Test clone
	parentDir := filepath.Join(tempDir, "clone-parent")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatalf("Failed to create parent directory %s: %v", parentDir, err)
	}
	destination := "cloned-repo"
	dest, err := clone(bareDir, parentDir, destination)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify destination
	expectedDest := filepath.Join(parentDir, destination)
	if dest != expectedDest {
		t.Errorf("Expected destination %q, got %q", expectedDest, dest)
	}

	// Verify cloned repository contains Project.json
	if _, err := os.Stat(filepath.Join(dest, "Project.json")); os.IsNotExist(err) {
		t.Errorf("Expected Project.json in %s, not found", dest)
	}

	// Verify .git directory exists
	if _, err := os.Stat(filepath.Join(dest, ".git")); os.IsNotExist(err) {
		t.Errorf("Expected .git directory in %s, not found", dest)
	}
}
