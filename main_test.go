package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cosm/types"

	"github.com/google/uuid"
)

// binaryPath holds the path to the compiled cosm binary
var binaryPath string

func TestMain(m *testing.M) {
	tempDir := os.TempDir()
	binaryPath = filepath.Join(tempDir, "cosm")

	cmd := exec.Command("go", "build", "-o", binaryPath, "main.go")
	if err := cmd.Run(); err != nil {
		println("Failed to build cosm binary:", err.Error())
		os.Exit(1)
	}

	exitCode := m.Run()
	// os.Remove(binaryPath) // Uncomment to clean up
	os.Exit(exitCode)
}

// runCommand runs the cosm binary with given args in a directory and returns output and error
func runCommand(t *testing.T, dir string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	return out.String(), errOut.String(), err
}

// checkOutput verifies the command output and exit code
func checkOutput(t *testing.T, stdout, stderr, expectedOutput string, err error, expectError bool, expectedExitCode int) {
	t.Helper()
	if expectError {
		if err == nil {
			t.Fatalf("Expected an error, got none (stdout: %q, stderr: %q)", stdout, stderr)
		}
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != expectedExitCode {
			t.Errorf("Expected exit code %d, got %v", expectedExitCode, err)
		}
	} else {
		if err != nil {
			t.Fatalf("Expected no error, got %v (stderr: %q)", err, stderr)
		}
	}
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q (stderr: %q)", expectedOutput, stdout, stderr)
	}
}

// checkProjectFile verifies the contents of Project.json, including UUID
func checkProjectFile(t *testing.T, file string, expected types.Project) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Failed to read Project.json: %v", err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("Failed to parse Project.json: %v", err)
	}
	if project.Name != expected.Name {
		t.Errorf("Expected Name %q, got %q", expected.Name, project.Name)
	}
	if project.Version != expected.Version {
		t.Errorf("Expected Version %q, got %q", expected.Version, project.Version)
	}
	if project.Language != expected.Language {
		t.Errorf("Expected Language %q, got %q", expected.Language, project.Language)
	}
	if len(project.Authors) != len(expected.Authors) {
		t.Errorf("Expected %d authors, got %d", len(expected.Authors), len(project.Authors))
	} else {
		for i, expAuthor := range expected.Authors {
			if project.Authors[i] != expAuthor {
				t.Errorf("Expected author %d: %q, got %q", i, expAuthor, project.Authors[i])
			}
		}
	}
	if len(project.Deps) != len(expected.Deps) {
		t.Errorf("Expected %d dependencies, got %d", len(expected.Deps), len(project.Deps))
	} else {
		for depName, expVersion := range expected.Deps {
			gotVersion, exists := project.Deps[depName]
			if !exists || gotVersion != expVersion {
				t.Errorf("Expected dep %q: %q, got %q", depName, expVersion, gotVersion)
			}
		}
	}
	if project.UUID == "" {
		t.Errorf("Expected non-empty UUID, got empty")
	} else if _, err := uuid.Parse(project.UUID); err != nil {
		t.Errorf("Expected valid UUID, got %q: %v", project.UUID, err)
	}
}

// setupTempGitConfig creates a temporary Git config file and sets mock values
func setupTempGitConfig(t *testing.T, tempDir string) string {
	t.Helper()
	tempGitConfig := filepath.Join(tempDir, "gitconfig")
	if err := os.WriteFile(tempGitConfig, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create temporary Git config file: %v", err)
	}
	os.Setenv("GIT_CONFIG_GLOBAL", tempGitConfig)
	t.Cleanup(func() { os.Unsetenv("GIT_CONFIG_GLOBAL") }) // Clean up after test

	// Set mock git config
	cmdName := exec.Command("git", "config", "--file", tempGitConfig, "user.name", "testuser")
	cmdEmail := exec.Command("git", "config", "--file", tempGitConfig, "user.email", "testuser@git.com")
	if err := cmdName.Run(); err != nil {
		t.Fatalf("Failed to set git user.name in temp config: %v", err)
	}
	if err := cmdEmail.Run(); err != nil {
		t.Fatalf("Failed to set git user.email in temp config: %v", err)
	}
	return tempGitConfig
}

// createBareRepo creates a bare Git repository in the given directory and returns its file:// URL
func createBareRepo(t *testing.T, dir string, name string) string {
	t.Helper()
	bareRepoPath := filepath.Join(dir, name)
	if err := exec.Command("git", "init", "--bare", bareRepoPath).Run(); err != nil {
		t.Fatalf("Failed to initialize bare Git repo at %s: %v", bareRepoPath, err)
	}
	return "file://" + bareRepoPath
}

// checkRegistriesFile verifies the contents of registries.json
func checkRegistriesFile(t *testing.T, registriesFile string, expected []types.Registry) {
	t.Helper()
	if _, err := os.Stat(registriesFile); os.IsNotExist(err) {
		t.Fatalf("registries.json was not created at %s", registriesFile)
	}
	data, err := os.ReadFile(registriesFile)
	if err != nil {
		t.Fatalf("Failed to read registries.json: %v", err)
	}
	var registries []types.Registry
	if err := json.Unmarshal(data, &registries); err != nil {
		t.Fatalf("Failed to parse registries.json: %v", err)
	}
	if len(registries) != len(expected) {
		t.Errorf("Expected %d registries, got %d", len(expected), len(registries))
	}
	for i, exp := range expected {
		if i >= len(registries) {
			break
		}
		got := registries[i]
		if got.Name != exp.Name {
			t.Errorf("Expected registry %d Name %q, got %q", i, exp.Name, got.Name)
		}
		if got.GitURL != exp.GitURL {
			t.Errorf("Expected registry %d GitURL %q, got %q", i, exp.GitURL, got.GitURL)
		}
		if len(got.Packages) != len(exp.Packages) {
			t.Errorf("Expected registry %d Packages len %d, got %d", i, len(exp.Packages), len(got.Packages))
		}
	}
}

func TestVersion(t *testing.T) {
	tempDir := t.TempDir()
	stdout, _, err := runCommand(t, tempDir, "--version")
	checkOutput(t, stdout, "", "cosm version 0.1.0\n", err, false, 0)
}

func TestStatus(t *testing.T) {
	tempDir := t.TempDir()
	stdout, _, err := runCommand(t, tempDir, "status")
	checkOutput(t, stdout, "", "Cosmic Status:\n  Orbit: Stable\n  Systems: All green\n  Pending updates: None\nRun 'cosm status' in a project directory for more details.\n", err, false, 0)
}

func TestActivateSuccess(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "cosm.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create mock cosm.json: %v", err)
	}
	stdout, _, err := runCommand(t, tempDir, "activate")
	checkOutput(t, stdout, "", "Activated current project\n", err, false, 0)
}

func TestActivateFailure(t *testing.T) {
	tempDir := t.TempDir()
	stdout, _, err := runCommand(t, tempDir, "activate")
	checkOutput(t, stdout, "", "Error: No project found in current directory (missing cosm.json)\n", err, true, 1)
}

func TestInit(t *testing.T) {
	tempDir := t.TempDir()
	packageName := "myproject"

	// Setup temporary Git config
	_ = setupTempGitConfig(t, tempDir)

	// Run the command
	stdout, stderr, err := runCommand(t, tempDir, "init", packageName)
	if err != nil {
		t.Fatalf("Command failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	expectedOutputPrefix := fmt.Sprintf("Initialized project '%s' with version v0.1.0 and UUID ", packageName)
	if !strings.HasPrefix(stdout, expectedOutputPrefix) {
		t.Errorf("Expected output to start with %q, got %q\nStderr: %s", expectedOutputPrefix, stdout, stderr)
	}

	// Expected author from temp config
	expectedAuthor := "[testuser]testuser@git.com"
	expectedAuthors := []string{expectedAuthor}

	// Check Project.json
	projectFile := filepath.Join(tempDir, "Project.json")
	expectedProject := types.Project{
		Name:     packageName,
		Authors:  expectedAuthors,
		Language: "",
		Version:  "v0.1.0",
		Deps:     make(map[string]string),
	}
	checkProjectFile(t, projectFile, expectedProject)
}

func TestInitDuplicate(t *testing.T) {
	tempDir := t.TempDir()
	packageName := "myproject"

	projectFile := filepath.Join(tempDir, "Project.json")
	initialProject := types.Project{
		Name:    "existing",
		UUID:    uuid.New().String(),
		Authors: []string{"[existing]existing@author.com"},
		Version: "v0.1.0",
		Deps:    make(map[string]string),
	}
	data, _ := json.MarshalIndent(initialProject, "", "  ")
	if err := os.WriteFile(projectFile, data, 0644); err != nil {
		t.Fatalf("Failed to create initial Project.json: %v", err)
	}
	dataBefore, _ := os.ReadFile(projectFile)

	stdout, _, err := runCommand(t, tempDir, "init", packageName)
	checkOutput(t, stdout, "", "Error: Project.json already exists in this directory\n", err, true, 1)

	dataAfter, _ := os.ReadFile(projectFile)
	if !bytes.Equal(dataBefore, dataAfter) {
		t.Errorf("Project.json changed unexpectedly")
	}
}

func TestRegistryInit(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"

	// Create a local bare Git repository
	gitURL := createBareRepo(t, tempDir, "origin.git")

	// Run the command and capture output
	stdout, stderr, err := runCommand(t, tempDir, "registry", "init", registryName, gitURL)
	if err != nil {
		t.Fatalf("Command failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	expectedOutput := fmt.Sprintf("Initialized registry '%s' with Git URL: %s\n", registryName, gitURL)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Verify registries.json
	registriesFile := filepath.Join(tempDir, ".cosm", "registries.json")
	checkRegistriesFile(t, registriesFile, []types.Registry{
		{Name: registryName, GitURL: gitURL, Packages: make(map[string][]string)},
	})
}
