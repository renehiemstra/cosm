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

	"cosm/commands"
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

// TestInit tests the cosm init command
func TestInit(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Test 1: Default version (v0.1.0)
	packageName := "myproject"
	packageDir := filepath.Join(tempDir, packageName)
	if err := os.Mkdir(packageDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir %s: %v", packageDir, err)
	}
	stdout, stderr, err := runCommand(t, packageDir, "init", packageName)
	checkOutput(t, stdout, stderr, fmt.Sprintf("Initialized project '%s' with version v0.1.0\n", packageName), err, false, 0)
	expectedProject := types.Project{
		Name:     packageName,
		Authors:  []string{"[testuser]testuser@git.com"},
		Language: "",
		Version:  "v0.1.0",
		Deps:     make(map[string]string),
	}
	checkProjectFile(t, filepath.Join(packageDir, "Project.json"), expectedProject)

	// Test 2: Specific version (v1.0.0)
	packageName2 := "myproject2"
	packageDir2 := filepath.Join(tempDir, packageName2)
	if err := os.Mkdir(packageDir2, 0755); err != nil {
		t.Fatalf("Failed to create package dir %s: %v", packageDir2, err)
	}
	stdout, stderr, err = runCommand(t, packageDir2, "init", packageName2, "v1.0.0")
	checkOutput(t, stdout, stderr, fmt.Sprintf("Initialized project '%s' with version v1.0.0\n", packageName2), err, false, 0)
	expectedProject2 := types.Project{
		Name:     packageName2,
		Authors:  []string{"[testuser]testuser@git.com"},
		Language: "",
		Version:  "v1.0.0",
		Deps:     make(map[string]string),
	}
	checkProjectFile(t, filepath.Join(packageDir2, "Project.json"), expectedProject2)
}

// TestInitDuplicate tests initializing a project when Project.json already exists
func TestInitDuplicate(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	packageName := "myproject"
	packageDir := filepath.Join(tempDir, packageName)
	if err := os.Mkdir(packageDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir %s: %v", packageDir, err)
	}
	stdout, stderr, err := runCommand(t, packageDir, "init", packageName)
	checkOutput(t, stdout, stderr, fmt.Sprintf("Initialized project '%s' with version v0.1.0\n", packageName), err, false, 0)

	// Try to initialize again
	stdout, stderr, err = runCommand(t, packageDir, "init", packageName)
	checkOutput(t, stdout, stderr, "", err, true, 1)
	expectedStderr := "Error: Project.json already exists in this directory\n"
	if stderr != expectedStderr {
		t.Errorf("Expected stderr %q, got %q", expectedStderr, stderr)
	}

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

// TestAddDependency tests the cosm add command
func TestAddDependency(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Setup registry
	registryName := "myreg"
	setupRegistry(t, tempDir, registryName)

	// Setup package to be added as a dependency
	packageName := "mypkg"
	packageVersion := "v0.1.0"
	_, packageGitURL := setupPackageWithGit(t, tempDir, packageName, packageVersion)
	addPackageToRegistry(t, tempDir, registryName, packageGitURL)

	// Initialize project
	projectDir := initPackage(t, tempDir, "myproject")

	// Add dependency to project
	stdout, stderr := addDependencyToProject(t, projectDir, packageName, packageVersion)
	expectedOutput := fmt.Sprintf("Added dependency '%s' %s from registry '%s' to project\n", packageName, packageVersion, registryName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Verify dependency in Project.json
	verifyProjectDependencies(t, filepath.Join(projectDir, "Project.json"), packageName, packageVersion)
}

func TestRegistryStatus(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Test 1: Successful status check with a valid registry
	registryName := "myreg"
	setupRegistry(t, tempDir, registryName)
	stdout, stderr, err := runCommand(t, tempDir, "registry", "status", registryName)
	expectedOutput := fmt.Sprintf("Registry Status for '%s':\n  No packages registered.\n", registryName)
	checkOutput(t, stdout, stderr, expectedOutput, err, false, 0)

	// Test 2: Error case with non-existent registry
	invalidRegistry := "nonexistent"
	stdout, stderr, err = runCommand(t, tempDir, "registry", "status", invalidRegistry)
	expectedStderr := fmt.Sprintf("Error: registry '%s' not found in registries.json\n", invalidRegistry)
	checkOutput(t, stdout, stderr, "", err, true, 1)
	if stderr != expectedStderr {
		t.Errorf("Expected stderr %q, got %q", expectedStderr, stderr)
	}
}

func TestRegistryInit(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	registryName := "myreg"
	gitURL, registryDir := setupRegistry(t, tempDir, registryName)

	// Verify output (duplicate init should fail)
	stdout, stderr, err := runCommand(t, tempDir, "registry", "init", registryName, createBareRepo(t, tempDir, "origin.git"))
	checkOutput(t, stdout, stderr, "", err, true, 1) // Changed expectedOutput to "" (empty stdout)

	// Verify stderr contains the error message
	expectedStderr := "Error: registry 'myreg' already exists\n"
	if stderr != expectedStderr {
		t.Errorf("Expected stderr %q, got %q", expectedStderr, stderr)
	}

	// Verify registries.json
	registriesFile := filepath.Join(tempDir, ".cosm", "registries", "registries.json")
	checkRegistriesFile(t, registriesFile, []string{registryName})

	// Verify registry metadata file
	registryMetaFile := filepath.Join(registryDir, "registry.json")
	checkRegistryMetaFile(t, registryMetaFile, types.Registry{
		Name:     registryName,
		GitURL:   gitURL,
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

// TestRegistryAdd tests the cosm registry add command
func TestRegistryAdd(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Setup dependency registry
	depRegName := "depReg"
	setupRegistry(t, tempDir, depRegName)

	// Setup dependency package
	depName := "dep1"
	depVersion := "v1.2.0"
	depUUID := "123e4567-e89b-12d3-a456-426614174001"
	depDir, depGitURL := setupPackageWithGit(t, tempDir, depName, depVersion)
	os.Chdir(depDir)
	os.WriteFile("Project.json", []byte(fmt.Sprintf(`{"name":"%s","uuid":"%s","version":"%s"}`, depName, depUUID, depVersion)), 0644)
	gitAddCommitPush(t, "Update Project.json")

	// Add dependency to depReg
	stdout, stderr := addPackageToRegistry(t, tempDir, depRegName, depGitURL)
	expectedOutput := fmt.Sprintf("Added package '%s' with version '%s' to registry '%s'\n", depName, depVersion, depRegName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Setup test registry
	registryName := "myreg"
	registryGitURL, registryDir := setupRegistry(t, tempDir, registryName)

	// Setup test package with dependency
	packageName := "mypkg"
	packageVersion := "v0.1.0"
	packageUUID := "123e4567-e89b-12d3-a456-426614174000"
	packageDir, packageGitURL := setupPackageWithGit(t, tempDir, packageName, packageVersion)
	os.Chdir(packageDir)
	os.WriteFile("Project.json", []byte(fmt.Sprintf(`{"name":"%s","uuid":"%s","version":"%s","deps":{"%s":"%s"}}`, packageName, packageUUID, packageVersion, depName, depVersion)), 0644)
	gitAddCommitPush(t, "Update Project.json with dep")

	// Add package to myreg
	stdout, stderr = addPackageToRegistry(t, tempDir, registryName, packageGitURL)
	expectedOutput = fmt.Sprintf("Added package '%s' with version '%s' to registry '%s'\n", packageName, packageVersion, registryName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}
	expectedStderrPrefix := fmt.Sprintf("No valid tags found; released version '%s' from Project.json to repository at '%s'", packageVersion, packageGitURL)
	if !strings.HasPrefix(stderr, expectedStderrPrefix) {
		t.Errorf("Expected stderr prefix %q, got %q", expectedStderrPrefix, stderr)
	}

	// Verify registry state
	registryMetaFile := filepath.Join(registryDir, "registry.json")
	checkRegistryMetaFile(t, registryMetaFile, types.Registry{
		Name:     registryName,
		GitURL:   registryGitURL,
		Packages: map[string]string{packageName: packageUUID},
	})

	// Verify package versions and specs
	versionsFile := filepath.Join(registryDir, "M", packageName, "versions.json")
	verifyVersionsJSON(t, versionsFile, []string{packageVersion})

	verifyPackageCloneExists(t, tempDir, packageUUID)

	specsFile := filepath.Join(registryDir, "M", packageName, packageVersion, "specs.json")
	verifySpecsJSON(t, specsFile, packageVersion)

	// Verify buildlist.json
	buildListFile := filepath.Join(registryDir, "M", packageName, packageVersion, "buildlist.json")
	data, err := os.ReadFile(buildListFile)
	if err != nil {
		t.Fatalf("Failed to read buildlist.json: %v", err)
	}
	var buildList types.BuildList
	if err := json.Unmarshal(data, &buildList); err != nil {
		t.Fatalf("Failed to parse buildlist.json: %v", err)
	}
	expectedKey := fmt.Sprintf("%s@v1", depUUID)
	if len(buildList.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency in buildlist.json, got %d", len(buildList.Dependencies))
	}
	if dep, exists := buildList.Dependencies[expectedKey]; !exists {
		t.Errorf("Expected dependency key %q in buildlist.json, not found", expectedKey)
	} else {
		if dep.Name != depName || dep.UUID != depUUID || dep.Version != depVersion {
			t.Errorf("Expected dependency %s@%s (UUID: %s), got %s@%s (UUID: %s)",
				depName, depVersion, depUUID, dep.Name, dep.Version, dep.UUID)
		}
	}

	// Test error: missing dependency
	invalidPackageName := "invalidpkg"
	invalidGitURL := createBareRepo(t, tempDir, "invalid.git")
	invalidDir := filepath.Join(tempDir, invalidPackageName)
	os.Mkdir(invalidDir, 0755)
	os.Chdir(invalidDir)
	os.WriteFile("Project.json", []byte(fmt.Sprintf(`{"name":"%s","uuid":"%s","version":"v0.1.0","deps":{"missing":"v1.0.0"}}`, invalidPackageName, uuid.NewString())), 0644)
	gitInitAddCommit(t, "Initial with missing dep")
	gitRemoteAddPush(t, invalidGitURL)
	stdout, stderr, err = runCommand(t, tempDir, "registry", "add", registryName, invalidGitURL)
	expectedStderr := "dependency 'missing@v1.0.0' not found in any registry"
	checkOutput(t, stdout, stderr, "", err, true, 1)
	if !strings.Contains(stderr, expectedStderr) {
		t.Errorf("Expected stderr containing %q, got %q", expectedStderr, stderr)
	}
}

// TestReleasePatch tests the cosm release --patch command
func TestRelease(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	registryName := "myreg"
	packageName := "mypkg"
	version := "v1.2.3"
	packageDir, registryDir := setupReleaseTestEnv(t, tempDir, registryName, packageName, version)

	// Execute release --patch
	patchRelease := "v1.2.4"
	executeAndVerifyRelease(t, registryDir, packageDir, packageName, []string{version}, patchRelease, "--patch")

	// Execute release --minor
	minorRelease := "v1.3.0"
	executeAndVerifyRelease(t, registryDir, packageDir, packageName, []string{version, patchRelease}, minorRelease, "--minor")

	// Execute release --major
	majorRelease := "v2.0.0"
	executeAndVerifyRelease(t, registryDir, packageDir, packageName, []string{version, patchRelease, minorRelease}, majorRelease, "--major")

	// Execute custom release
	customRelease := "v3.1.2"
	executeAndVerifyRelease(t, registryDir, packageDir, packageName, []string{version, patchRelease, minorRelease, majorRelease}, customRelease, customRelease)
}

func TestMakePackageAvailable(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Setup registry and package
	registryName := "myreg"
	packageName := "mypkg"
	packageVersion := "v0.1.0"
	_, packageGitURL := setupPackageWithGit(t, tempDir, packageName, packageVersion)
	setupRegistry(t, tempDir, registryName)
	addPackageToRegistry(t, tempDir, registryName, packageGitURL)

	// Capture the initial branch of the clone
	clonePath := filepath.Join(tempDir, ".cosm", "clones")
	var specs types.Specs
	specsFile := filepath.Join(tempDir, ".cosm", "registries", registryName, "M", packageName, packageVersion, "specs.json")
	data, err := os.ReadFile(specsFile)
	if err != nil {
		t.Fatalf("Failed to read specs.json: %v", err)
	}
	if err := json.Unmarshal(data, &specs); err != nil {
		t.Fatalf("Failed to parse specs.json: %v", err)
	}
	cloneDir := filepath.Join(clonePath, specs.UUID)
	if err := os.Chdir(cloneDir); err != nil {
		t.Fatalf("Failed to change to clone dir: %v", err)
	}
	initialBranchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	initialBranchOutput, err := initialBranchCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get initial branch: %v", err)
	}
	initialBranch := strings.TrimSpace(string(initialBranchOutput))

	// Test success case
	err = commands.MakePackageAvailable(filepath.Join(tempDir, ".cosm"), registryName, packageName, packageVersion)
	if err != nil {
		t.Fatalf("MakePackageAvailable failed: %v", err)
	}

	// Verify destination directory
	destPath := filepath.Join(tempDir, ".cosm", "packages", packageName, specs.SHA1)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Errorf("Destination directory %s not created", destPath)
	}

	// Verify clone is back on the initial branch after success
	if err := os.Chdir(cloneDir); err != nil {
		t.Fatalf("Failed to change to clone dir: %v", err)
	}
	successBranchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	successBranchOutput, err := successBranchCmd.Output()
	if err != nil {
		t.Fatalf("Failed to check branch after success: %v", err)
	}
	successBranch := strings.TrimSpace(string(successBranchOutput))
	if successBranch != initialBranch {
		t.Errorf("Expected clone to revert to initial branch %q after success, got %q", initialBranch, successBranch)
	}

	// Clean up the packages directory to force recreation in error case
	packagesDir := filepath.Join(tempDir, ".cosm", "packages")
	if err := os.RemoveAll(packagesDir); err != nil {
		t.Fatalf("Failed to remove packages directory: %v", err)
	}

	// Test error case: make destination directory unwritable to simulate failure
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to recreate packages dir: %v", err)
	}
	if err := os.Chmod(packagesDir, 0500); err != nil { // Read+execute only, no write
		t.Fatalf("Failed to make packages dir unwritable: %v", err)
	}
	defer os.Chmod(packagesDir, 0755) // Restore permissions

	// Verify packages dir is unwritable
	if err := os.Mkdir(filepath.Join(packagesDir, "test"), 0755); err == nil {
		t.Fatalf("Expected packages dir to be unwritable, but could create subdirectory")
	}

	err = commands.MakePackageAvailable(filepath.Join(tempDir, ".cosm"), registryName, packageName, packageVersion)
	if err == nil {
		t.Errorf("Expected MakePackageAvailable to fail due to unwritable directory")
	}

	// Verify clone is still reverted after error
	if err := os.Chdir(cloneDir); err != nil {
		t.Fatalf("Failed to change to clone dir: %v", err)
	}
	errorBranchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	errorBranchOutput, err := errorBranchCmd.Output()
	if err != nil {
		t.Fatalf("Failed to check branch after error: %v", err)
	}
	finalBranch := strings.TrimSpace(string(errorBranchOutput))
	if finalBranch != initialBranch {
		t.Errorf("Expected clone to revert to initial branch %q after error, got %q", initialBranch, finalBranch)
	}
}

/////////////////////// SETUP HELPER FUNCTIONS ///////////////////////

// testProject represents a package configuration for testing with a Git URL
type testProject struct {
	types.Project
	GitURL string
}

// setupRegistry initializes a registry with a bare Git remote using cosm registry init
func setupRegistry(t *testing.T, tempDir, registryName string) (string, string) {
	gitURL := createBareRepo(t, tempDir, registryName+".git")
	_, _, err := runCommand(t, tempDir, "registry", "init", registryName, gitURL)
	if err != nil {
		t.Fatalf("Failed to init registry '%s': %v", registryName, err)
	}
	return gitURL, filepath.Join(tempDir, ".cosm", "registries", registryName)
}

// initPackage initializes a package with cosm init and verifies the result
func initPackage(t *testing.T, tempDir, packageName string, version ...string) string {
	t.Helper()
	packageDir := filepath.Join(tempDir, packageName)
	if err := os.Mkdir(packageDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir %s: %v", packageDir, err)
	}

	// Run cosm init with optional version
	args := []string{"init", packageName}
	if len(version) > 0 {
		args = append(args, version[0])
	}
	stdout, stderr, err := runCommand(t, packageDir, args...)
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr)
	}

	// Determine expected version
	expectedVersion := "v0.1.0"
	if len(version) > 0 {
		expectedVersion = version[0]
	}
	expectedOutput := fmt.Sprintf("Initialized project '%s' with version %s\n", packageName, expectedVersion)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Verify Project.json
	expectedProject := types.Project{
		Name:     packageName,
		Authors:  []string{"[testuser]testuser@git.com"},
		Language: "",
		Version:  expectedVersion,
		Deps:     make(map[string]string),
	}
	checkProjectFile(t, filepath.Join(packageDir, "Project.json"), expectedProject)

	return packageDir
}

// setupPackageWithGit creates a package with a Git remote and a specified version
func setupPackageWithGit(t *testing.T, tempDir, packageName, version string) (string, string) {
	t.Helper()
	packageDir := initPackage(t, tempDir, packageName, version)
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

// setupTestEnv sets up a temporary environment with a Git config
func setupTestEnv(t *testing.T) (tempDir string, cleanup func()) {
	tempDir = t.TempDir()
	os.Setenv("HOME", tempDir)
	_ = setupTempGitConfig(t, tempDir)
	cleanup = func() { os.Unsetenv("HOME") }
	return tempDir, cleanup
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

// setupRegistryWithPackages sets up a registry with pre-initialized packages
func setupRegistryWithPackages(t *testing.T, tempDir, registryName string, packages []testProject) string {
	_, registryDir := setupRegistry(t, tempDir, registryName)
	for _, pkg := range packages {
		if pkg.GitURL == "" {
			t.Fatalf("Package %s has no GitURL; must be pre-initialized with setupPackageWithGit", pkg.Name)
		}
		addPackageToRegistry(t, tempDir, registryName, pkg.GitURL)
	}
	return registryDir
}

// setupReleaseTestEnv prepares a package and registry for release testing
func setupReleaseTestEnv(t *testing.T, tempDir, registryName, packageName, initialVersion string) (string, string) {
	t.Helper()

	// Initialize package with specified version and Git remote
	packageDir, gitURL := setupPackageWithGit(t, tempDir, packageName, initialVersion)

	// Load the project's current state from Project.json
	project := loadProjectFile(t, filepath.Join(packageDir, "Project.json"))

	// Define test package with loaded project and Git URL
	pkg := testProject{
		Project: project,
		GitURL:  gitURL,
	}

	// Register package in the registry
	registryDir := setupRegistryWithPackages(t, tempDir, registryName, []testProject{pkg})

	return packageDir, registryDir
}

func releasePackage(t *testing.T, packageDir, releaseArg string) (string, string) {
	var args []string
	switch releaseArg {
	case "--patch", "--minor", "--major":
		args = []string{"release", releaseArg}
	default:
		if strings.HasPrefix(releaseArg, "v") {
			args = []string{"release", releaseArg}
		} else {
			t.Fatalf("Invalid release argument: %s. Use --patch, --minor, --major, or v<version>", releaseArg)
		}
	}
	stdout, stderr, err := runCommand(t, packageDir, args...)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Release command failed with exit code %d: stdout: %q, stderr: %q", exitErr.ExitCode(), stdout, stderr)
		} else {
			t.Fatalf("Release command failed: %v, stdout: %q, stderr: %q", err, stdout, stderr)
		}
	}
	return stdout, stderr
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

// loadProjectFile reads and parses Project.json from a given file path
func loadProjectFile(t *testing.T, projectFile string) types.Project {
	t.Helper()
	data, err := os.ReadFile(projectFile)
	if err != nil {
		t.Fatalf("Failed to read Project.json: %v", err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("Failed to parse Project.json: %v", err)
	}
	if project.Deps == nil {
		project.Deps = make(map[string]string)
	}
	return project
}

// Helper function to add, commit, and push changes
func gitAddCommitPush(t *testing.T, message string) {
	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", message).Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
	if err := exec.Command("git", "push", "origin", "main").Run(); err != nil {
		t.Fatalf("Failed to git push: %v", err)
	}
}

// Helper function for invalid package setup
func gitInitAddCommit(t *testing.T, message string) {
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Fatalf("Failed to git init: %v", err)
	}
	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", message).Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
}

func gitRemoteAddPush(t *testing.T, remoteURL string) {
	if err := exec.Command("git", "remote", "add", "origin", remoteURL).Run(); err != nil {
		t.Fatalf("Failed to add remote: %v", err)
	}
	if err := exec.Command("git", "push", "origin", "main").Run(); err != nil {
		t.Fatalf("Failed to git push: %v", err)
	}
}

/////////////////////// Check HELPER FUNCTIONS ///////////////////////

func verifyProjectDependencies(t *testing.T, projectFile, packageName, expectedVersion string) {
	project := loadProjectFile(t, projectFile)
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

// checkRegistryMetaFile verifies the contents of registry.json against an expected registry
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
	if expected.GitURL != "" && registry.GitURL != expected.GitURL {
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
	for pkgName, expectedUUID := range expected.Packages {
		if gotUUID, exists := registry.Packages[pkgName]; !exists {
			t.Errorf("Expected package %q in registry, not found", pkgName)
		} else if gotUUID != expectedUUID {
			t.Errorf("Expected UUID %q for package %q, got %q", expectedUUID, pkgName, gotUUID)
		}
	}
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

// checkProjectFile verifies the contents of Project.json against an expected project
func checkProjectFile(t *testing.T, file string, expected types.Project) {
	t.Helper()
	project := loadProjectFile(t, file)

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

// verifyProjectVersion checks the Project.json version
func verifyProjectVersion(t *testing.T, projectFile, expectedVersion string) {
	data, err := os.ReadFile(projectFile)
	if err != nil {
		t.Fatalf("Failed to read Project.json: %v", err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("Failed to parse Project.json: %v", err)
	}
	if project.Version != expectedVersion {
		t.Errorf("Expected Project.json version %q, got %q", expectedVersion, project.Version)
	}
}

// verifyGitTag checks if the specified tag exists in the Git repo
func verifyGitTag(t *testing.T, packageDir, tag string) {
	if err := os.Chdir(packageDir); err != nil {
		t.Fatalf("Failed to change to package dir: %v", err)
	}
	fetchCmd := exec.Command("git", "fetch", "origin")
	if err := fetchCmd.Run(); err != nil {
		t.Fatalf("Failed to fetch from origin: %v", err)
	}
	tagCmd := exec.Command("git", "tag", "-l", tag)
	tagOutput, err := tagCmd.Output()
	if err != nil {
		t.Fatalf("Failed to list tags: %v", err)
	}
	if !strings.Contains(string(tagOutput), tag) {
		t.Errorf("Expected tag %q to exist, got %q", tag, string(tagOutput))
	}
}

// verifyRegistryUpdates checks the registry's versions.json and specs.json
func verifyRegistryUpdates(t *testing.T, versionsFile string, specs types.Specs, newVersion string, previousVersions []string) {
	data, err := os.ReadFile(versionsFile)
	if err != nil {
		t.Fatalf("Failed to read versions.json: %v", err)
	}
	var versions []string
	if err := json.Unmarshal(data, &versions); err != nil {
		t.Fatalf("Failed to parse versions.json: %v", err)
	}
	expectedVersions := append(previousVersions, newVersion)
	if len(versions) != len(expectedVersions) {
		t.Errorf("Expected %d versions, got %d: %v", len(expectedVersions), len(versions), versions)
	}
	for _, expected := range expectedVersions {
		found := false
		for _, v := range versions {
			if v == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected %q in versions.json, got %v", expected, versions)
		}
	}

	if specs.Version != newVersion {
		t.Errorf("Expected specs.json version %q, got %q", newVersion, specs.Version)
	}
	if specs.SHA1 == "" {
		t.Errorf("Expected non-empty SHA1 in specs.json, got empty")
	}
}

// verifySHA1Matches ensures the SHA1 in specs matches the Git tag's SHA1
func verifySHA1Matches(t *testing.T, packageDir, tag string, specs types.Specs) {
	if err := os.Chdir(packageDir); err != nil {
		t.Fatalf("Failed to change to package dir: %v", err)
	}
	sha1Output, err := exec.Command("git", "rev-list", "-n", "1", tag).Output()
	if err != nil {
		t.Fatalf("Failed to get SHA1 for tag %q: %v", tag, err)
	}
	expectedSHA1 := strings.TrimSpace(string(sha1Output))
	if specs.SHA1 != expectedSHA1 {
		t.Errorf("Expected SHA1 %q in specs.json, got %q", expectedSHA1, specs.SHA1)
	}
}

func executeAndVerifyRelease(t *testing.T, registryDir, packageDir, packageName string, previousVersions []string, newVersion, incrementOrVersion string) {
	stdout, stderr := releasePackage(t, packageDir, incrementOrVersion)
	expectedOutput := fmt.Sprintf("Released version '%s' for project '%s'\n", newVersion, packageName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Verify release results
	verifyRelease(t, packageDir, registryDir, packageName, newVersion, previousVersions)
}

// verifyRelease orchestrates the release verification steps
func verifyRelease(t *testing.T, packageDir, registryDir, packageName, newVersion string, previousVersions []string) {
	// Read specs.json once
	specsFile := filepath.Join(registryDir, "M", packageName, newVersion, "specs.json")
	data, err := os.ReadFile(specsFile)
	if err != nil {
		t.Fatalf("Failed to read specs.json: %v", err)
	}
	var specs types.Specs
	if err := json.Unmarshal(data, &specs); err != nil {
		t.Fatalf("Failed to parse specs.json: %v", err)
	}
	verifyProjectVersion(t, filepath.Join(packageDir, "Project.json"), newVersion)
	verifyGitTag(t, packageDir, newVersion)
	verifyRegistryUpdates(t, filepath.Join(registryDir, "M", packageName, "versions.json"), specs, newVersion, previousVersions)
	verifySHA1Matches(t, packageDir, newVersion, specs)
}
