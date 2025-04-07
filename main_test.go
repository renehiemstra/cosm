package main

import (
	"bytes"
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		t.Errorf("Expected output %q, got %q", expectedOutput, stdout)
	}
}

// setupRegistriesFile creates a registries.json with given registries
func setupRegistriesFile(t *testing.T, dir string, registries []struct {
	Name        string              `json:"name"`
	GitURL      string              `json:"giturl"`
	Packages    map[string][]string `json:"packages,omitempty"`
	LastUpdated time.Time           `json:"last_updated,omitempty"`
}) string {
	t.Helper()
	cosmDir := filepath.Join(dir, ".cosm")
	if err := os.MkdirAll(cosmDir, 0755); err != nil {
		t.Fatalf("Failed to create .cosm directory: %v", err)
	}
	registriesFile := filepath.Join(cosmDir, "registries.json")
	data, err := json.MarshalIndent(registries, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal registries: %v", err)
	}
	if err := os.WriteFile(registriesFile, data, 0644); err != nil {
		t.Fatalf("Failed to write registries.json: %v", err)
	}
	return registriesFile
}

// checkRegistriesFile verifies the contents of registries.json
func checkRegistriesFile(t *testing.T, file string, expected []struct {
	Name        string              `json:"name"`
	GitURL      string              `json:"giturl"`
	Packages    map[string][]string `json:"packages,omitempty"`
	LastUpdated time.Time           `json:"last_updated,omitempty"`
}) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Failed to read registries.json: %v", err)
	}
	var registries []struct {
		Name        string              `json:"name"`
		GitURL      string              `json:"giturl"`
		Packages    map[string][]string `json:"packages,omitempty"`
		LastUpdated time.Time           `json:"last_updated,omitempty"`
	}
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
		if got.Name != exp.Name || got.GitURL != exp.GitURL {
			t.Errorf("Expected registry %d: {Name: %q, GitURL: %q}, got {Name: %q, GitURL: %q}",
				i, exp.Name, exp.GitURL, got.Name, got.GitURL)
		}
		if len(got.Packages) != len(exp.Packages) {
			t.Errorf("Expected %d packages for %s, got %d", len(exp.Packages), exp.Name, len(got.Packages))
		}
		for pkgName, expVersions := range exp.Packages {
			gotVersions, exists := got.Packages[pkgName]
			if !exists {
				t.Errorf("Package %s not found in registry %s", pkgName, exp.Name)
				continue
			}
			if len(gotVersions) != len(expVersions) {
				t.Errorf("Expected %d versions for %s in %s, got %d", len(expVersions), pkgName, exp.Name, len(gotVersions))
			}
			for j, v := range expVersions {
				if j >= len(gotVersions) || gotVersions[j] != v {
					t.Errorf("Expected version %d for %s in %s: %q, got %q", j, pkgName, exp.Name, v, gotVersions[j])
				}
			}
		}
		if !exp.LastUpdated.IsZero() && got.LastUpdated.IsZero() {
			t.Errorf("Expected LastUpdated to be set for %s, got zero", exp.Name)
		}
	}
}

// checkProjectFile verifies the contents of Project.json
func checkProjectFile(t *testing.T, file string, expected struct {
	Name         string             `json:"name"`
	Version      string             `json:"version"`
	Dependencies []types.Dependency `json:"dependencies,omitempty"`
}) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Failed to read Project.json: %v", err)
	}
	var project struct {
		Name         string             `json:"name"`
		Version      string             `json:"version"`
		Dependencies []types.Dependency `json:"dependencies,omitempty"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("Failed to parse Project.json: %v", err)
	}
	if project.Name != expected.Name || project.Version != expected.Version {
		t.Errorf("Expected Project {Name: %q, Version: %q}, got {Name: %q, Version: %q}",
			expected.Name, expected.Version, project.Name, project.Version)
	}
	if len(project.Dependencies) != len(expected.Dependencies) {
		t.Errorf("Expected %d dependencies, got %d", len(expected.Dependencies), len(project.Dependencies))
	}
	for i, expDep := range expected.Dependencies {
		if i >= len(project.Dependencies) {
			break
		}
		gotDep := project.Dependencies[i]
		if gotDep.Name != expDep.Name || gotDep.Version != expDep.Version {
			t.Errorf("Expected dependency %d: {Name: %q, Version: %q}, got {Name: %q, Version: %q}",
				i, expDep.Name, expDep.Version, gotDep.Name, gotDep.Version)
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

// TestInit tests the cosm init command with isolated Git config
func TestInit(t *testing.T) {
	tempDir := t.TempDir()
	packageName := "myproject"

	// Create a temporary Git config file
	tempGitConfig := filepath.Join(tempDir, "gitconfig")
	if err := os.WriteFile(tempGitConfig, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create temporary Git config file: %v", err)
	}

	// Set GIT_CONFIG_GLOBAL to isolate Git config changes
	os.Setenv("GIT_CONFIG_GLOBAL", tempGitConfig)
	defer os.Unsetenv("GIT_CONFIG_GLOBAL") // Clean up after test

	// Set mock git config in the temporary file
	cmdName := exec.Command("git", "config", "--file", tempGitConfig, "user.name", "testuser")
	cmdEmail := exec.Command("git", "config", "--file", tempGitConfig, "user.email", "testuser@git.com")
	if err := cmdName.Run(); err != nil {
		t.Fatalf("Failed to set git user.name in temp config: %v", err)
	}
	if err := cmdEmail.Run(); err != nil {
		t.Fatalf("Failed to set git user.email in temp config: %v", err)
	}

	// Run the command
	stdout, _, err := runCommand(t, tempDir, "init", packageName)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	expectedOutputPrefix := fmt.Sprintf("Initialized project '%s' with version v0.1.0 and UUID ", packageName)
	if !strings.HasPrefix(stdout, expectedOutputPrefix) {
		t.Errorf("Expected output to start with %q, got %q", expectedOutputPrefix, stdout)
	}

	// Determine expected author based on the temporary git config
	expectedAuthor := "[unknown]unknown@author.com"
	name, errName := exec.Command("git", "config", "--file", tempGitConfig, "user.name").Output()
	email, errEmail := exec.Command("git", "config", "--file", tempGitConfig, "user.email").Output()
	if errName == nil && errEmail == nil && len(name) > 0 && len(email) > 0 {
		expectedAuthor = fmt.Sprintf("[%s]%s", strings.TrimSpace(string(name)), strings.TrimSpace(string(email)))
	}
	expectedAuthors := []string{expectedAuthor}

	// Check the created Project.json
	projectFile := filepath.Join(tempDir, "Project.json")
	expectedProject := types.Project{
		Name:     packageName,
		Authors:  expectedAuthors,
		Language: "", // Unspecified by default
		Version:  "v0.1.0",
		Deps:     make(map[string]string),
	}
	data, err := os.ReadFile(projectFile)
	if err != nil {
		t.Fatalf("Failed to read Project.json: %v", err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("Failed to parse Project.json: %v", err)
	}

	// Verify fields
	if project.Name != expectedProject.Name {
		t.Errorf("Expected Name %q, got %q", expectedProject.Name, project.Name)
	}
	if project.Version != expectedProject.Version {
		t.Errorf("Expected Version %q, got %q", expectedProject.Version, project.Version)
	}
	if project.Language != expectedProject.Language {
		t.Errorf("Expected Language %q, got %q", expectedProject.Language, project.Language)
	}
	if len(project.Authors) != len(expectedProject.Authors) || project.Authors[0] != expectedProject.Authors[0] {
		t.Errorf("Expected Authors %v, got %v", expectedProject.Authors, project.Authors)
	}
	if len(project.Deps) != 0 {
		t.Errorf("Expected empty Deps, got %v", project.Deps)
	}
	if project.UUID == "" {
		t.Errorf("Expected non-empty UUID, got empty")
	} else {
		if _, err := uuid.Parse(project.UUID); err != nil {
			t.Errorf("Expected valid UUID, got %q: %v", project.UUID, err)
		}
	}
}

func TestInitDuplicate(t *testing.T) {
	tempDir := t.TempDir()
	packageName := "myproject"

	projectFile := filepath.Join(tempDir, "Project.json")
	initialProject := struct {
		Name         string             `json:"name"`
		Version      string             `json:"version"`
		Dependencies []types.Dependency `json:"dependencies,omitempty"`
	}{Name: "existing", Version: "v0.1.0"}
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

// TestRegistryInit tests the cosm registry init command
func TestRegistryInit(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"

	// Create a local bare Git repository (simulating a remote origin)
	bareRepoPath := filepath.Join(tempDir, "origin.git")
	if err := exec.Command("git", "init", "--bare", bareRepoPath).Run(); err != nil {
		t.Fatalf("Failed to initialize bare Git repo: %v", err)
	}
	gitURL := "file://" + bareRepoPath

	// Run the command and capture output for debugging
	stdout, stderr, err := runCommand(t, tempDir, "registry", "init", registryName, gitURL)
	if err != nil {
		t.Fatalf("Command failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	expectedOutput := fmt.Sprintf("Initialized registry '%s' with Git URL: %s\n", registryName, gitURL)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Verify registries.json exists and contains the expected data
	registriesFile := filepath.Join(tempDir, ".cosm", "registries.json")
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
	if len(registries) != 1 {
		t.Errorf("Expected 1 registry, got %d", len(registries))
	} else {
		reg := registries[0]
		if reg.Name != registryName {
			t.Errorf("Expected Name %q, got %q", registryName, reg.Name)
		}
		if reg.GitURL != gitURL {
			t.Errorf("Expected GitURL %q, got %q", gitURL, reg.GitURL)
		}
		if len(reg.Packages) != 0 {
			t.Errorf("Expected empty Packages map, got %+v", reg.Packages)
		}
	}
}
