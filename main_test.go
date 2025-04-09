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
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	packageName := "myproject"
	packageDir := setupPackage(t, tempDir, packageName)

	// Capture output
	stdout, stderr, err := runCommand(t, packageDir, "init", packageName)
	if err == nil {
		t.Fatalf("Expected an error for duplicate init, got none (stdout: %q, stderr: %q)", stdout, stderr)
	}
	expectedOutput := "Error: Project.json already exists in this directory\n"
	checkOutput(t, stdout, stderr, expectedOutput, err, true, 1)

	// Verify Project.json
	projectFile := filepath.Join(packageDir, "Project.json")
	expectedProject := types.Project{
		Name:     packageName,
		Authors:  []string{"[testuser]testuser@git.com"},
		Language: "",
		Version:  "v0.1.0",
		Deps:     make(map[string]string),
	}
	checkProjectFile(t, projectFile, expectedProject)
}

func TestInitDuplicate(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	packageName := "myproject"
	packageDir := setupPackage(t, tempDir, packageName)

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
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Setup registry and package
	registryName := "myreg"
	setupRegistry(t, tempDir, registryName)
	packageName := "mypkg"
	_, packageGitURL := setupPackageWithGit(t, tempDir, packageName)
	addPackageToRegistry(t, tempDir, registryName, packageGitURL) // Adds v0.1.0

	// Init project
	projectDir := setupPackage(t, tempDir, "myproject")

	// Add dependency to project
	stdout, stderr := addDependencyToProject(t, projectDir, packageName, "v0.1.0")
	expectedOutput := fmt.Sprintf("Added dependency '%s' v0.1.0 from registry '%s' to project\n", packageName, registryName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Verify Project.json
	projectFile := filepath.Join(projectDir, "Project.json")
	verifyProjectDependencies(t, projectFile, packageName, "v0.1.0")
}

func TestRegistryInit(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	registryName := "myreg"
	gitURL, registryDir := setupRegistry(t, tempDir, registryName)

	// Verify output (duplicate init should fail)
	stdout, stderr, err := runCommand(t, tempDir, "registry", "init", registryName, createBareRepo(t, tempDir, "origin.git"))
	checkOutput(t, stdout, stderr, "Error: Registry 'myreg' already exists\n", err, true, 1)

	// Verify registries.json
	registriesFile := filepath.Join(tempDir, ".cosm", "registries", "registries.json")
	checkRegistriesFile(t, registriesFile, []string{registryName})

	// Verify registry metadata file
	registryMetaFile := filepath.Join(registryDir, "registry.json")
	checkRegistryMetaFile(t, registryMetaFile, types.Registry{
		Name:     registryName,
		GitURL:   gitURL, // Use the actual gitURL from setupRegistry
		Packages: make(map[string]string),
	})
}

func TestRegistryInitSuccess(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get original directory: %v", err)
	}

	registryName := "myreg"
	gitURL := createBareRepo(t, tempDir, "registry.git")
	stdout, stderr, err := runCommand(t, tempDir, "registry", "init", registryName, gitURL)
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr)
	}
	expectedOutput := fmt.Sprintf("Initialized registry '%s' with Git URL: %s\n", registryName, gitURL)
	checkOutput(t, stdout, stderr, expectedOutput, err, false, 0)

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
}

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
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	registryName := "myreg"
	_, registryDir := setupRegistry(t, tempDir, registryName)

	packageName := "mypkg"
	_, packageGitURL := setupPackageWithGit(t, tempDir, packageName)

	// Add package to registry (should tag v0.1.0)
	stdout, stderr := addPackageToRegistry(t, tempDir, registryName, packageGitURL)
	expectedOutputPrefix := fmt.Sprintf("Added package '%s' with UUID '", packageName)
	if !strings.HasPrefix(stdout, expectedOutputPrefix) {
		t.Errorf("Expected stdout to start with %q, got %q\nStderr: %s", expectedOutputPrefix, stdout, stderr)
	}
	expectedStderrPrefix := fmt.Sprintf("No valid tags found; released version 'v0.1.0' from Project.json to repository at '%s'", packageGitURL)
	if !strings.HasPrefix(stderr, expectedStderrPrefix) {
		t.Errorf("Expected stderr to start with %q, got %q", expectedStderrPrefix, stderr)
	}

	// Verify registry metadata
	registryMetaFile := filepath.Join(registryDir, "registry.json")
	packageUUID := verifyRegistryMetadata(t, registryMetaFile, packageName)

	// Verify versions.json contains v0.1.0
	versionsFile := filepath.Join(registryDir, "M", packageName, "versions.json")
	verifyVersionsJSON(t, versionsFile, []string{"v0.1.0"})

	// Verify package clone exists
	verifyPackageCloneExists(t, tempDir, packageUUID)

	// Verify specs.json for v0.1.0
	specsFile := filepath.Join(registryDir, "M", packageName, "v0.1.0", "specs.json")
	verifySpecsJSON(t, specsFile, "v0.1.0")
}

/////////////////////// HELPER FUNCTIONS ///////////////////////

// setupRegistry initializes a registry with a bare Git remote using cosm registry init
func setupRegistry(t *testing.T, tempDir, registryName string) (string, string) {
	gitURL := createBareRepo(t, tempDir, registryName+".git")
	_, _, err := runCommand(t, tempDir, "registry", "init", registryName, gitURL)
	if err != nil {
		t.Fatalf("Failed to init registry '%s': %v", registryName, err)
	}
	return gitURL, filepath.Join(tempDir, ".cosm", "registries", registryName)
}

// setupPackageWithGit creates a package with a Git remote and no tags
func setupPackageWithGit(t *testing.T, tempDir, packageName string) (string, string) {
	packageDir := setupPackage(t, tempDir, packageName)
	if err := os.Chdir(packageDir); err != nil {
		t.Fatalf("Failed to change to package dir %s: %v", packageDir, err)
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
	return packageDir, bareRepoURL
}

// addPackageToRegistry adds a package to a registry using cosm registry add
func addPackageToRegistry(t *testing.T, tempDir, registryName, packageGitURL string) (string, string) {
	stdout, stderr, err := runCommand(t, tempDir, "registry", "add", registryName, packageGitURL)
	if err != nil {
		t.Fatalf("Failed to add package to registry '%s': %v\nStderr: %s", registryName, err, stderr)
	}
	return stdout, stderr
}

// verifyRegistryMetadata checks the registry metadata in registry.json
func verifyRegistryMetadata(t *testing.T, registryMetaFile, packageName string) string {
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
	return packageUUID
}

// addDependencyToProject adds a dependency to a project using cosm add
func addDependencyToProject(t *testing.T, projectDir, packageName, version string) (string, string) {
	cmd := exec.Command(binaryPath, "add", packageName+"@"+version)
	cmd.Dir = projectDir
	cmd.Stdin = strings.NewReader("\n") // Empty input for single registry case
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to add dependency '%s@%s': %v\nStderr: %s", packageName, version, err, stderr.String())
	}
	return stdout.String(), stderr.String()
}

// verifyProjectDependencies checks the Deps field in Project.json
func verifyProjectDependencies(t *testing.T, projectFile, packageName, expectedVersion string) {
	data, err := os.ReadFile(projectFile)
	if err != nil {
		t.Fatalf("Failed to read Project.json: %v", err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("Failed to parse Project.json: %v", err)
	}
	if version, exists := project.Deps[packageName]; !exists || version != expectedVersion {
		t.Errorf("Expected dependency %s:%s, got %v", packageName, expectedVersion, project.Deps)
	}
}

// verifyVersionsJSON checks the versions.json file for expected versions
func verifyVersionsJSON(t *testing.T, versionsFile string, expectedVersions []string) {
	data, err := os.ReadFile(versionsFile)
	if err != nil {
		t.Fatalf("Failed to read versions.json: %v", err)
	}
	var versions []string
	if err := json.Unmarshal(data, &versions); err != nil {
		t.Fatalf("Failed to parse versions.json: %v", err)
	}
	if len(versions) != len(expectedVersions) {
		t.Errorf("Expected %d versions, got %d: %v", len(expectedVersions), len(versions), versions)
	}
	for i, expected := range expectedVersions {
		if i >= len(versions) || versions[i] != expected {
			t.Errorf("Expected version %q at index %d, got %q", expected, i, versions[i])
		}
	}
}

// verifyPackageCloneExists ensures the package clone exists in the clones directory
func verifyPackageCloneExists(t *testing.T, tempDir, packageUUID string) {
	clonePath := filepath.Join(tempDir, ".cosm", "clones", packageUUID)
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		t.Errorf("Package clone not found at %s", clonePath)
	}
}

// verifySpecsJSON checks the specs.json file for a specific version
func verifySpecsJSON(t *testing.T, specsFile, expectedVersion string) {
	data, err := os.ReadFile(specsFile)
	if err != nil {
		t.Fatalf("Failed to read specs.json: %v", err)
	}
	var specs types.Specs
	if err := json.Unmarshal(data, &specs); err != nil {
		t.Fatalf("Failed to parse specs.json: %v", err)
	}
	if specs.Version != expectedVersion {
		t.Errorf("Expected specs.json version %q, got %q", expectedVersion, specs.Version)
	}
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

// setupTestEnv sets up a temporary environment with a Git config
func setupTestEnv(t *testing.T) (tempDir string, cleanup func()) {
	tempDir = t.TempDir()
	os.Setenv("HOME", tempDir)
	_ = setupTempGitConfig(t, tempDir)
	cleanup = func() { os.Unsetenv("HOME") }
	return tempDir, cleanup
}

// setupPackage creates a package directory and initializes it with cosm init
func setupPackage(t *testing.T, tempDir, packageName string) string {
	packageDir := filepath.Join(tempDir, packageName)
	if err := os.Mkdir(packageDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir %s: %v", packageDir, err)
	}
	_, _, err := runCommand(t, packageDir, "init", packageName)
	if err != nil {
		t.Fatalf("Failed to init package %s: %v", packageName, err)
	}
	return packageDir
}

// createBareRepo creates a bare Git repository and returns its file:// URL
func createBareRepo(t *testing.T, dir string, name string) string {
	t.Helper()
	bareRepoPath := filepath.Join(dir, name)
	if err := exec.Command("git", "init", "--bare", bareRepoPath).Run(); err != nil {
		t.Fatalf("Failed to initialize bare Git repo at %s: %v", bareRepoPath, err)
	}
	return "file://" + bareRepoPath
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
