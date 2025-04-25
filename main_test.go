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
	initPackage(t, tempDir, packageName)

	// Test 2: Specific version (v1.0.0)
	packageName2 := "myproject2"
	initPackage(t, tempDir, packageName2, "v1.0.0")
}

// TestInitDuplicate tests initializing a project when Project.json already exists
func TestInitDuplicate(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Initialize project
	packageName := "myproject"
	packageDir := filepath.Join(tempDir, packageName)
	initPackage(t, tempDir, packageName)

	// Verify the file didn’t change
	dataBefore, err := os.ReadFile(filepath.Join(packageDir, "Project.json"))
	if err != nil {
		t.Fatalf("Failed to read Project.json after first init: %v", err)
	}

	// Try to initialize again
	stdout, stderr, err := runCommand(t, packageDir, "init", packageName, "v1.0.0")
	checkOutput(t, stdout, stderr, "", err, true, 1)
	expectedStderr := "Error: Project.json already exists in this directory\n"
	if stderr != expectedStderr {
		t.Errorf("Expected stderr %q, got %q", expectedStderr, stderr)
	}

	dataAfter, err := os.ReadFile(filepath.Join(packageDir, "Project.json"))
	if err != nil {
		t.Fatalf("Failed to read Project.json after second init: %v", err)
	}

	// Verify the file didn’t change
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
