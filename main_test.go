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

// Global projectRoot set at package initialization
var projectRoot string

// binaryPath holds the path to the compiled cosm binary
var binaryPath string

func init() {
	var err error
	projectRoot, err = filepath.Abs(".")
	if err != nil {
		panic("Failed to get absolute project root: " + err.Error())
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); os.IsNotExist(err) {
		panic("Project root " + projectRoot + " does not contain go.mod")
	}
}

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

// createBareRepo creates a bare Git repository in the given directory and returns its file:// URL
func createBareRepo(t *testing.T, dir string, name string) string {
	t.Helper()
	bareRepoPath := filepath.Join(dir, name)
	if err := exec.Command("git", "init", "--bare", bareRepoPath).Run(); err != nil {
		t.Fatalf("Failed to initialize bare Git repo at %s: %v", bareRepoPath, err)
	}
	return "file://" + bareRepoPath
}

// initPackage initializes a package with cosm init and Git setup
func initPackage(t *testing.T, dir, packageName string, tags ...string) string {
	t.Helper()
	packageDir := filepath.Join(dir, packageName)
	if err := os.Mkdir(packageDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir %s: %v", packageDir, err)
	}
	if err := os.Chdir(packageDir); err != nil {
		t.Fatalf("Failed to change to package dir %s: %v", packageDir, err)
	}
	_, _, err := runCommand(t, packageDir, "init", packageName)
	if err != nil {
		t.Fatalf("Failed to init package %s: %v", packageName, err)
	}
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Fatalf("Failed to init Git repo for %s: %v", packageName, err)
	}
	if err := exec.Command("git", "add", "Project.json").Run(); err != nil {
		t.Fatalf("Failed to add Project.json for %s: %v", packageName, err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatalf("Failed to commit for %s: %v", packageName, err)
	}
	for _, tag := range tags {
		if err := exec.Command("git", "tag", tag).Run(); err != nil {
			t.Fatalf("Failed to tag %s for %s: %v", tag, packageName, err)
		}
	}
	return packageDir
}

// setupPackageRepo sets up a package repo with a bare remote and returns the bare repo URL
func setupPackageRepo(t *testing.T, tempDir, packageName string, tags ...string) string {
	t.Helper()
	packageDir := initPackage(t, tempDir, packageName, tags...)
	bareRepoURL := createBareRepo(t, tempDir, packageName+".git")
	if err := os.Chdir(packageDir); err != nil {
		t.Fatalf("Failed to change to package dir %s: %v", packageDir, err)
	}
	if err := exec.Command("git", "remote", "add", "origin", bareRepoURL).Run(); err != nil {
		t.Fatalf("Failed to add remote for %s: %v", packageName, err)
	}
	if err := exec.Command("git", "push", "origin", "main").Run(); err != nil {
		t.Fatalf("Failed to push main for %s: %v", packageName, err)
	}
	for _, tag := range tags {
		if err := exec.Command("git", "push", "origin", tag).Run(); err != nil {
			t.Fatalf("Failed to push tag %s for %s: %v", tag, packageName, err)
		}
	}
	return bareRepoURL
}

// checkRegistriesFile (updated path)
func checkRegistriesFile(t *testing.T, registriesFile string, expected []string) {
	t.Helper()
	if _, err := os.Stat(registriesFile); os.IsNotExist(err) {
		t.Fatalf("registries.json was not created at %s", registriesFile)
	}
	data, err := os.ReadFile(registriesFile)
	if err != nil {
		t.Fatalf("Failed to read registries.json: %v", err)
	}
	var registryNames []string
	if err := json.Unmarshal(data, &registryNames); err != nil {
		t.Fatalf("Failed to parse registries.json: %v", err)
	}
	if len(registryNames) != len(expected) {
		t.Errorf("Expected %d registry names, got %d", len(expected), len(registryNames))
	}
	for i, exp := range expected {
		if i >= len(registryNames) {
			break
		}
		if registryNames[i] != exp {
			t.Errorf("Expected registry name %d %q, got %q", i, exp, registryNames[i])
		}
	}
}

// checkRegistryMetaFile (updated path)
func checkRegistryMetaFile(t *testing.T, registryMetaFile string, expected types.Registry) {
	t.Helper()
	if _, err := os.Stat(registryMetaFile); os.IsNotExist(err) {
		t.Fatalf("registry.json was not created at %s", registryMetaFile)
	}
	data, err := os.ReadFile(registryMetaFile)
	if err != nil {
		t.Fatalf("Failed to read registry.json: %v", err)
	}
	var registry types.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("Failed to parse registry.json: %v", err)
	}
	if registry.Name != expected.Name {
		t.Errorf("Expected Name %q, got %q", expected.Name, registry.Name)
	}
	if registry.GitURL != expected.GitURL {
		t.Errorf("Expected GitURL %q, got %q", expected.GitURL, registry.GitURL)
	}
	if registry.UUID == "" {
		t.Errorf("Expected non-empty UUID, got empty")
	} else if _, err := uuid.Parse(registry.UUID); err != nil {
		t.Errorf("Expected valid UUID, got %q: %v", registry.UUID, err)
	}
	if len(registry.Packages) != len(expected.Packages) {
		t.Errorf("Expected Packages len %d, got %d", len(expected.Packages), len(registry.Packages))
	}
}

func TestVersion(t *testing.T) {
	tempDir := t.TempDir()
	stdout, _, err := runCommand(t, tempDir, "--version")
	checkOutput(t, stdout, "", "cosm version 0.1.0\n", err, false, 0)
}

func TestStatus(t *testing.T) {
}

func TestActivateSuccess(t *testing.T) {
}

func TestActivateFailure(t *testing.T) {
}

func TestInit(t *testing.T) {
	tempDir := t.TempDir()
	packageName := "myproject"

	// Override HOME to isolate .cosm in tempDir
	os.Setenv("HOME", tempDir)
	t.Cleanup(func() { os.Unsetenv("HOME") })

	// Setup temporary Git config
	_ = setupTempGitConfig(t, tempDir)

	// Initialize the package and capture output
	packageDir := filepath.Join(tempDir, packageName)
	if err := os.Mkdir(packageDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir %s: %v", packageDir, err)
	}
	stdout, stderr, err := runCommand(t, packageDir, "init", packageName)
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
	projectFile := filepath.Join(packageDir, "Project.json")
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

	// Override HOME to isolate .cosm in tempDir
	os.Setenv("HOME", tempDir)
	t.Cleanup(func() { os.Unsetenv("HOME") })

	// Setup temporary Git config
	_ = setupTempGitConfig(t, tempDir)

	// Initialize the project once
	packageDir := initPackage(t, tempDir, packageName)

	// Try to initialize again
	stdout, stderr, err := runCommand(t, packageDir, "init", packageName)
	checkOutput(t, stdout, stderr, "Error: Project.json already exists in this directory\n", err, true, 1)

	// Verify the file didn’t change
	dataBefore, err := os.ReadFile(filepath.Join(packageDir, "Project.json"))
	if err != nil {
		t.Fatalf("Failed to read Project.json after first init: %v", err)
	}
	dataAfter, err := os.ReadFile(filepath.Join(packageDir, "Project.json"))
	if err != nil {
		t.Fatalf("Failed to read Project.json after second init: %v", err)
	}
	if !bytes.Equal(dataBefore, dataAfter) {
		t.Errorf("Project.json changed unexpectedly")
	}
}

func TestAddDependency(t *testing.T) {

	tempDir := t.TempDir()

	// Override HOME to isolate .cosm in tempDir
	os.Setenv("HOME", tempDir)
	t.Cleanup(func() { os.Unsetenv("HOME") })

	// Setup temporary Git config
	_ = setupTempGitConfig(t, tempDir)

	// Setup registry and package
	registryName := "myreg"
	packageName := "mypkg"
	registryGitURL := createBareRepo(t, tempDir, "registry.git")
	_, _, err := runCommand(t, tempDir, "registry", "init", registryName, registryGitURL)
	if err != nil {
		t.Fatalf("Failed to init registry: %v", err)
	}
	packageGitURL := setupPackageRepo(t, tempDir, packageName, "v1.0.0")
	_, _, err = runCommand(t, tempDir, "registry", "add", registryName, packageGitURL)
	if err != nil {
		t.Fatalf("Failed to add package: %v", err)
	}

	// Init project
	projectDir := initPackage(t, tempDir, "myproject")

	// Simulate user input for single registry case
	cmd := exec.Command(binaryPath, "add", packageName+"@v1.0.0")
	cmd.Dir = projectDir
	cmd.Stdin = strings.NewReader("\n") // Empty input since only one registry
	stdout, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = errOut
	err = cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, errOut.String())
	}
	expectedOutput := fmt.Sprintf("Added dependency '%s' v1.0.0 from registry '%s' to project\n", packageName, registryName)
	if stdout.String() != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout.String(), errOut.String())
	}

	// Verify Project.json
	data, err := os.ReadFile(filepath.Join(projectDir, "Project.json"))
	if err != nil {
		t.Fatalf("Failed to read Project.json: %v", err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("Failed to parse Project.json: %v", err)
	}
	if version, exists := project.Deps[packageName]; !exists || version != "v1.0.0" {
		t.Errorf("Expected dependency %s:v1.0.0, got %v", packageName, project.Deps)
	}
}

func TestRegistryInit(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"

	// Create a local bare Git repository
	gitURL := createBareRepo(t, tempDir, "origin.git")

	// Override HOME to isolate .cosm in tempDir
	os.Setenv("HOME", tempDir)
	t.Cleanup(func() { os.Unsetenv("HOME") })

	// Run the command and capture output
	stdout, stderr, err := runCommand(t, tempDir, "registry", "init", registryName, gitURL)
	if err != nil {
		t.Fatalf("Command failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	expectedOutput := fmt.Sprintf("Initialized registry '%s' with Git URL: %s\n", registryName, gitURL)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Verify registries.json (list of names)
	registriesFile := filepath.Join(tempDir, ".cosm", "registries", "registries.json")
	checkRegistriesFile(t, registriesFile, []string{registryName})

	// Verify registry metadata file
	registryMetaFile := filepath.Join(tempDir, ".cosm", "registries", registryName, "registry.json")
	checkRegistryMetaFile(t, registryMetaFile, types.Registry{
		Name:     registryName,
		GitURL:   gitURL,
		Packages: make(map[string]string),
	})
}

func TestRegistryInitSuccess(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get original directory: %v", err)
	}

	os.Setenv("HOME", tempDir)
	t.Cleanup(func() { os.Unsetenv("HOME") })
	_ = setupTempGitConfig(t, tempDir)

	registryGitURL := createBareRepo(t, tempDir, "registry.git")
	registryName := "myreg"

	_, stderr, err := runCommand(t, tempDir, "registry", "init", registryName, registryGitURL)
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr)
	}

	// Check current working directory reverted to originalDir
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if currentDir != originalDir {
		t.Errorf("Expected current directory %s, got %s", originalDir, currentDir)
	}

	// Check registrySubDir exists
	registrySubDir := filepath.Join(tempDir, ".cosm", "registries", registryName)
	if _, err := os.Stat(registrySubDir); os.IsNotExist(err) {
		t.Errorf("Expected registry directory %s to exist", registrySubDir)
	}

	// Check registry.json exists
	registryMetaFile := filepath.Join(registrySubDir, "registry.json")
	if _, err := os.Stat(registryMetaFile); os.IsNotExist(err) {
		t.Errorf("Expected registry.json at %s to exist", registryMetaFile)
	}

	// Verify Git repository state (optional, requires cloning or inspecting remote)
	// Could add a git log check if remote is accessible in test setup
}

// TestRegistryStatus tests the cosm registry status command
func TestRegistryStatus(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"
	packageName := "mypkg"

	// Override HOME to isolate .cosm in tempDir
	os.Setenv("HOME", tempDir)
	t.Cleanup(func() { os.Unsetenv("HOME") })

	// Create a bare registry repo
	registryGitURL := createBareRepo(t, tempDir, "registry.git")
	_, _, err := runCommand(t, tempDir, "registry", "init", registryName, registryGitURL)
	if err != nil {
		t.Fatalf("Failed to init registry: %v", err)
	}

	// Setup package repo with tags and add to registry
	packageGitURL := setupPackageRepo(t, tempDir, packageName, "v1.0.0", "v1.1.0")
	_, _, err = runCommand(t, tempDir, "registry", "add", registryName, packageGitURL)
	if err != nil {
		t.Fatalf("Failed to add package: %v", err)
	}

	// Run the status command
	stdout, stderr, err := runCommand(t, tempDir, "registry", "status", registryName)
	if err != nil {
		t.Fatalf("Command failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	// Verify output
	registryMetaFile := filepath.Join(tempDir, ".cosm", "registries", registryName, "registry.json")
	data, err := os.ReadFile(registryMetaFile)
	if err != nil {
		t.Fatalf("Failed to read registry.json: %v", err)
	}
	var registry types.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("Failed to parse registry.json: %v", err)
	}
	packageUUID := registry.Packages[packageName]
	expectedOutput := fmt.Sprintf("Registry Status for '%s':\n  Packages:\n    - %s (UUID: %s)\n", registryName, packageName, packageUUID)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}
}

func TestRegistryAdd(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"
	packageName := "mypkg"

	// Override HOME to isolate .cosm in tempDir
	os.Setenv("HOME", tempDir)
	t.Cleanup(func() { os.Unsetenv("HOME") })

	// Setup temporary Git config
	_ = setupTempGitConfig(t, tempDir)

	// Create a bare registry repo
	registryGitURL := createBareRepo(t, tempDir, "registry.git")
	_, _, err := runCommand(t, tempDir, "registry", "init", registryName, registryGitURL)
	if err != nil {
		t.Fatalf("Failed to init registry: %v", err)
	}

	// Setup package repo without tags
	packageDir := filepath.Join(tempDir, packageName)
	if err := os.Mkdir(packageDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir %s: %v", packageDir, err)
	}
	if err := os.Chdir(packageDir); err != nil {
		t.Fatalf("Failed to change to package dir %s: %v", packageDir, err)
	}
	_, _, err = runCommand(t, packageDir, "init", packageName)
	if err != nil {
		t.Fatalf("Failed to init package %s: %v", packageName, err)
	}
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Fatalf("Failed to init Git repo for %s: %v", packageName, err)
	}
	if err := exec.Command("git", "add", "Project.json").Run(); err != nil {
		t.Fatalf("Failed to add Project.json for %s: %v", packageName, err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatalf("Failed to commit for %s: %v", packageName, err)
	}
	bareRepoURL := createBareRepo(t, tempDir, packageName+".git")
	if err := exec.Command("git", "remote", "add", "origin", bareRepoURL).Run(); err != nil {
		t.Fatalf("Failed to add remote for %s: %v", packageName, err)
	}
	if err := exec.Command("git", "push", "origin", "main").Run(); err != nil {
		t.Fatalf("Failed to push main for %s: %v", packageName, err)
	}
	packageGitURL := bareRepoURL

	// Add package to registry (should tag v0.1.0)
	stdout, stderr, err := runCommand(t, tempDir, "registry", "add", registryName, packageGitURL)
	if err != nil {
		t.Fatalf("Command failed on first add: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	expectedOutputPrefix := fmt.Sprintf("Added package '%s' with UUID '", packageName)
	if !strings.HasPrefix(stdout, expectedOutputPrefix) {
		t.Errorf("Expected stdout to start with %q, got %q\nStderr: %s", expectedOutputPrefix, stdout, stderr)
	}
	expectedStderrPrefix := fmt.Sprintf("No valid tags found; released version 'v0.1.0' from Project.json to repository at '%s'", packageGitURL)
	if !strings.HasPrefix(stderr, expectedStderrPrefix) {
		t.Errorf("Expected stderr to start with %q, got %q", expectedStderrPrefix, stderr)
	}

	// Verify registry metadata
	registryMetaFile := filepath.Join(tempDir, ".cosm", "registries", registryName, "registry.json")
	data, err := os.ReadFile(registryMetaFile)
	if err != nil {
		t.Fatalf("Failed to read registry.json: %v", err)
	}
	var registry types.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("Failed to parse registry.json: %v", err)
	}
	if len(registry.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(registry.Packages))
	}
	packageUUID, exists := registry.Packages[packageName]
	if !exists {
		t.Errorf("Expected package '%s' in registry, not found", packageName)
	}

	// Verify versions.json contains v0.1.0
	versionsFile := filepath.Join(tempDir, ".cosm", "registries", registryName, "M", packageName, "versions.json")
	data, err = os.ReadFile(versionsFile)
	if err != nil {
		t.Fatalf("Failed to read versions.json: %v", err)
	}
	var versions []string
	if err := json.Unmarshal(data, &versions); err != nil {
		t.Fatalf("Failed to parse versions.json: %v", err)
	}
	if len(versions) != 1 || versions[0] != "v0.1.0" {
		t.Errorf("Expected versions.json to contain ['v0.1.0'], got %v", versions)
	}

	// Verify package clone exists
	clonePath := filepath.Join(tempDir, ".cosm", "clones", packageUUID)
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		t.Errorf("Package clone not found at %s", clonePath)
	}

	// Verify specs.json for v0.1.0
	specsFile := filepath.Join(tempDir, ".cosm", "registries", registryName, "M", packageName, "v0.1.0", "specs.json")
	data, err = os.ReadFile(specsFile)
	if err != nil {
		t.Fatalf("Failed to read specs.json: %v", err)
	}
	var specs types.Specs
	if err := json.Unmarshal(data, &specs); err != nil {
		t.Fatalf("Failed to parse specs.json: %v", err)
	}
	if specs.Version != "v0.1.0" {
		t.Errorf("Expected specs.json version 'v0.1.0', got %q", specs.Version)
	}
}
