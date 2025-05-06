package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cosm/commands"
	"cosm/types"
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
	expectedStderr := fmt.Sprintf("Error: failed to validate registry '%s': registry '%s' not found in registries.json\n", invalidRegistry, invalidRegistry)
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
		Packages: make(map[string]types.PackageInfo),
	})
}

func TestRegistryDelete(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	tests := []struct {
		name           string
		registryName   string
		input          string // Stdin input for confirmation
		args           []string
		expectError    bool
		expectedStderr string
		verifyResults  func(t *testing.T, registriesDir string)
	}{
		{
			name:         "success_with_force",
			registryName: "myreg1",
			args:         []string{"registry", "delete", "myreg1", "--force"},
			verifyResults: func(t *testing.T, registriesDir string) {
				verifyRegistryDeleted(t, registriesDir, "myreg1")
			},
		},
		{
			name:         "success_with_confirmation",
			registryName: "myreg2",
			input:        "y\n",
			args:         []string{"registry", "delete", "myreg2"},
			verifyResults: func(t *testing.T, registriesDir string) {
				verifyRegistryDeleted(t, registriesDir, "myreg2")
			},
		},
		{
			name:           "error_non_existent_registry",
			registryName:   "nonexistent",
			args:           []string{"registry", "delete", "nonexistent"},
			expectError:    true,
			expectedStderr: "Error: registry 'nonexistent' not found in registries.json",
			verifyResults: func(t *testing.T, registriesDir string) {
				// Verify registries.json contains only myreg3 (since myreg1, myreg2 are not set up)
				checkRegistriesFile(t, filepath.Join(registriesDir, "registries.json"), []string{"myreg3"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup registries for this subtest
			registriesDir := filepath.Join(tempDir, ".cosm", "registries")
			if tt.name == "success_with_force" {
				setupRegistry(t, tempDir, "myreg1")
			} else if tt.name == "success_with_confirmation" {
				setupRegistry(t, tempDir, "myreg2")
			} else if tt.name == "error_non_existent_registry" {
				setupRegistry(t, tempDir, "myreg3")
			}

			cmd := exec.Command(binaryPath, tt.args...)
			cmd.Dir = tempDir
			if tt.input != "" {
				cmd.Stdin = strings.NewReader(tt.input)
			}
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got none")
				}
				if !strings.Contains(stderr.String(), tt.expectedStderr) {
					t.Errorf("Expected stderr containing %q, got %q", tt.expectedStderr, stderr.String())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v\nStderr: %s", err, stderr.String())
				}
				var expectedOutput string
				if tt.name == "success_with_confirmation" {
					expectedOutput = fmt.Sprintf("Are you sure you want to delete registry '%s'? [y/N]: Deleted registry '%s'\n", tt.registryName, tt.registryName)
				} else {
					expectedOutput = fmt.Sprintf("Deleted registry '%s'\n", tt.registryName)
				}
				if stdout.String() != expectedOutput {
					t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout.String(), stderr.String())
				}
			}

			tt.verifyResults(t, registriesDir)
		})
	}
}

func TestRegistryClone(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create registry
	registryName := "myreg"
	gitURL, registryDir := setupRegistry(t, tempDir, registryName)

	// Create a package and add to registry
	packageName := "mypkg"
	version := "v1.2.3"
	packageDir, packageGitURL := setupPackageWithGit(t, tempDir, packageName, version)
	tagPackageVersion(t, packageDir, version) // Ensure version is tagged
	addPackageToRegistry(t, tempDir, registryName, packageGitURL)

	// Delete the registry
	deleteRegistry(t, tempDir, registryName, true)
	verifyRegistryDeleted(t, filepath.Join(tempDir, ".cosm", "registries"), registryName)

	// Clone the registry
	cmd := exec.Command(binaryPath, "registry", "clone", gitURL)
	cmd.Dir = tempDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("Unexpected error: %v\nStderr: %s", err, stderr.String())
	}

	// Verify output
	expectedOutput := fmt.Sprintf("Cloned registry '%s' from %s\n", registryName, gitURL)
	if stdout.String() != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout.String(), stderr.String())
	}

	// Verify results
	project := loadProjectFile(t, filepath.Join(packageDir, "Project.json"))
	verifyRegistryCloned(t, tempDir, registryDir, registryName, map[string]types.PackageInfo{
		packageName: {UUID: project.UUID, GitURL: packageGitURL},
	})
	checkRegistriesFile(t, filepath.Join(tempDir, ".cosm", "registries", "registries.json"), []string{registryName})
	verifyRegistryPackage(t, registryDir, packageName, project.UUID, packageGitURL, version)

	// Verify Git commit in registry
	verifyRemoteUpdated(t, tempDir, registryDir, fmt.Sprintf("Added package %s version %s", packageName, version))
}

func TestRelease(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a package with bare remote
	packageName := "mypkg"
	version := "v1.2.3"
	packageDir, _ := setupPackageWithGit(t, tempDir, packageName, version)

	// Execute release --patch
	newVersion := "v1.2.4"
	stdout, stderr := releasePackage(t, packageDir, "--patch")
	expectedOutput := fmt.Sprintf("Released version '%s' for project '%s'\n", newVersion, packageName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}
	verifyProjectVersion(t, filepath.Join(packageDir, "Project.json"), newVersion)
	verifyGitTag(t, packageDir, newVersion)

	// Execute release --minor
	newVersion = "v1.3.0"
	stdout, stderr = releasePackage(t, packageDir, "--minor")
	expectedOutput = fmt.Sprintf("Released version '%s' for project '%s'\n", newVersion, packageName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}
	verifyProjectVersion(t, filepath.Join(packageDir, "Project.json"), newVersion)
	verifyGitTag(t, packageDir, newVersion)

	// Execute release --major
	newVersion = "v2.0.0"
	stdout, stderr = releasePackage(t, packageDir, "--major")
	expectedOutput = fmt.Sprintf("Released version '%s' for project '%s'\n", newVersion, packageName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}
	verifyProjectVersion(t, filepath.Join(packageDir, "Project.json"), newVersion)
	verifyGitTag(t, packageDir, newVersion)
}

func TestRegistryAddFirstTime(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create registry
	registryName := "myreg"
	_, registryDir := setupRegistry(t, tempDir, registryName)

	// Create a package and add to registry
	packageName := "mypkg"
	packageVersion := "v1.2.3"
	packageDir, gitURL := setupPackageWithGit(t, tempDir, packageName, packageVersion)
	stdout, stderr := addPackageToRegistry(t, tempDir, registryName, gitURL)

	// Verify output
	expectedOutput := fmt.Sprintf("Added package '%s' to registry '%s'\n", packageName, registryName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Check that versions.json does not exist
	versionsFile := filepath.Join(registryDir, strings.ToUpper(string(packageName[0])), packageName, "versions.json")
	if _, err := os.Stat(versionsFile); !os.IsNotExist(err) {
		t.Errorf("Expected no versions.json for '%s' (no released versions), found file", packageName)
	}

	// Verify registry.json
	project := loadProjectFile(t, filepath.Join(packageDir, "Project.json"))
	checkRegistryMetaFile(t, filepath.Join(registryDir, "registry.json"), types.Registry{
		Name: registryName,
		Packages: map[string]types.PackageInfo{
			packageName: {UUID: project.UUID, GitURL: gitURL},
		},
	})

	// Verify package clone exists
	clonePath := filepath.Join(tempDir, ".cosm", "clones", project.UUID)
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		t.Errorf("Expected package clone at %s, not found", clonePath)
	}

	verifyRemoteUpdated(t, tempDir, registryDir, fmt.Sprintf("Added package %s", packageName))
}

func TestRegistryAdd(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create registry
	registryName := "myreg"
	_, registryDir := setupRegistry(t, tempDir, registryName)

	// Create a package and add to registry
	packageName := "mypkg"
	packageVersion := "v1.2.3"
	packageDir, gitURL := setupPackageWithGit(t, tempDir, packageName, packageVersion)

	// Execute releases
	releasePackage(t, packageDir, "v1.2.4")
	releasePackage(t, packageDir, "--patch")
	releasePackage(t, packageDir, "--minor")
	releasePackage(t, packageDir, "--major")

	// add package to registry
	stdout, stderr := addPackageToRegistry(t, tempDir, registryName, gitURL)

	// Verify output
	expectedOutput := fmt.Sprintf("Added package '%s' to registry '%s'\n", packageName, registryName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// check that versions v1.2.4, v1.2.5, v1.3.0, v2.0.0 exists
	project := loadProjectFile(t, filepath.Join(packageDir, "Project.json"))
	expectedVersions := []string{"v1.2.4", "v1.2.5", "v1.3.0", "v2.0.0"}
	verifyVersionsJSON(t, filepath.Join(registryDir, strings.ToUpper(string(packageName[0])), packageName, "versions.json"), expectedVersions)
	for _, version := range expectedVersions {
		verifyRegistryPackage(t, registryDir, packageName, project.UUID, gitURL, version)
	}
}

func TestRegistryAddSingle(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create registry
	registryName := "myreg"
	_, registryDir := setupRegistry(t, tempDir, registryName)

	// Create a package and add to registry
	packageName := "mypkg"
	packageVersion := "v1.2.3"
	packageDir, gitURL := setupPackageWithGit(t, tempDir, packageName, packageVersion)

	// add package to registry
	stdout, stderr := addPackageToRegistry(t, tempDir, registryName, gitURL)

	// Verify output
	expectedOutput := fmt.Sprintf("Added package '%s' to registry '%s'\n", packageName, registryName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Execute releases
	releasePackage(t, packageDir, "v1.2.3")
	releasePackage(t, packageDir, "--patch")
	releasePackage(t, packageDir, "--minor")
	releasePackage(t, packageDir, "--major")

	// // Add a single version
	newVersion := "v1.3.0"
	_, stderr, err := runCommand(t, tempDir, "registry", "add", registryName, packageName, newVersion)
	if err != nil {
		t.Errorf("Unexpected error: %v\nStderr: %s", err, stderr)
	}

	// check that versions v1.3.0 exists
	project := loadProjectFile(t, filepath.Join(packageDir, "Project.json"))
	expectedVersions := []string{"v1.3.0"}
	verifyVersionsJSON(t, filepath.Join(registryDir, strings.ToUpper(string(packageName[0])), packageName, "versions.json"), expectedVersions)
	for _, version := range expectedVersions {
		verifyRegistryPackage(t, registryDir, packageName, project.UUID, gitURL, version)
	}
}

func TestRegistryRm(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create registry
	registryName := "myreg"
	_, registryDir := setupRegistry(t, tempDir, registryName)

	// Create a package and add to registry
	packageName := "mypkg"
	packageVersion := "v1.2.3"
	packageDir, gitURL := setupPackageWithGit(t, tempDir, packageName, packageVersion)

	// Execute releases
	patchRelease := "v1.2.4"
	releasePackage(t, packageDir, patchRelease)

	// add package to registry
	stdout, stderr := addPackageToRegistry(t, tempDir, registryName, gitURL)

	// Verify output
	expectedOutput := fmt.Sprintf("Added package '%s' to registry '%s'\n", packageName, registryName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Remove the only version
	stdout, stderr = removeFromRegistry(t, tempDir, registryName, packageName, patchRelease)

	// Verify version removed, package remains
	project := loadProjectFile(t, filepath.Join(packageDir, "Project.json"))
	verifyVersionsJSON(t, filepath.Join(registryDir, strings.ToUpper(string(packageName[0])), packageName, "versions.json"), []string{})
	verifyPackageRemoved(t, registryDir, packageName, patchRelease)
	verifyPackageInRegistry(t, registryDir, packageName, project.UUID, gitURL)
	verifyRemoteUpdated(t, tempDir, registryDir, fmt.Sprintf("Removed version '%s' of package '%s'", patchRelease, packageName))

	// Remove entire package
	stdout, stderr = removeFromRegistry(t, tempDir, registryName, packageName, "")

	// Verify package completely removed
	verifyPackageRemoved(t, registryDir, packageName, "")
	checkRegistryMetaFile(t, filepath.Join(registryDir, "registry.json"), types.Registry{
		Name:     registryName,
		Packages: make(map[string]types.PackageInfo),
	})
	verifyRemoteUpdated(t, tempDir, registryDir, fmt.Sprintf("Removed package '%s'", packageName))
}

func TestAddDependency(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Setup registry
	registryName := "myreg"
	setupRegistry(t, tempDir, registryName)

	// Setup package to be added as a dependency
	packageName := "mypkg"
	packageVersion := "v0.1.0"
	packageDir, packageGitURL := setupPackageWithGit(t, tempDir, packageName, packageVersion)
	releasePackage(t, packageDir, packageVersion)
	addPackageToRegistry(t, tempDir, registryName, packageGitURL)

	// // Initialize project
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

// TestAddDependencyNoVersion tests the cosm add command when no version is specified
func TestAddDependencyNoVersion(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Setup registry
	registryName := "myreg"
	setupRegistry(t, tempDir, registryName)

	// Setup package with multiple versions
	packageName := "mypkg"
	packageVersions := []string{"v1.0.0", "v1.1.0", "v1.2.0"}
	packageDir, packageGitURL := setupPackageWithGit(t, tempDir, packageName, packageVersions[0])
	for _, version := range packageVersions {
		releasePackage(t, packageDir, version)
	}
	addPackageToRegistry(t, tempDir, registryName, packageGitURL)

	// Initialize project
	projectDir := initPackage(t, tempDir, "myproject")

	// Add dependency to project without specifying version
	stdout, stderr := addDependencyToProject(t, projectDir, packageName, "")
	expectedOutput := fmt.Sprintf("Added dependency '%s' %s from registry '%s' to project\n", packageName, packageVersions[len(packageVersions)-1], registryName)
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Verify dependency in Project.json
	verifyProjectDependencies(t, filepath.Join(projectDir, "Project.json"), packageName, packageVersions[len(packageVersions)-1])
}

func TestRmDependency(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Setup registry and package
	registryName := "myreg"
	setupRegistry(t, tempDir, registryName)
	packageName := "mypkg"
	packageVersion := "v0.1.0"
	packageDir, packageGitURL := setupPackageWithGit(t, tempDir, packageName, packageVersion)
	releasePackage(t, packageDir, packageVersion)
	addPackageToRegistry(t, tempDir, registryName, packageGitURL)

	// Initialize project and add dependency
	projectDir := initPackage(t, tempDir, "myproject")
	addDependencyToProject(t, projectDir, packageName, packageVersion)

	// Remove dependency
	removeDependencyFromProject(t, projectDir, packageName)

	// Verify dependency is removed
	project := loadProjectFile(t, filepath.Join(projectDir, "Project.json"))
	if _, exists := project.Deps[packageName]; exists {
		t.Errorf("Dependency '%s' still exists in Project.json", packageName)
	}

	// Test error: remove non-existent dependency
	_, stderr, err := runCommand(t, projectDir, "rm", "nonexistent")
	if err == nil {
		t.Errorf("Expected error when removing non-existent dependency, got none")
	}
	expectedStderr := "Error: dependency 'nonexistent' not found in project\n"
	if stderr != expectedStderr {
		t.Errorf("Expected stderr %q, got %q", expectedStderr, stderr)
	}
}

func TestRmDependencyMultipleMatches(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Setup registry
	registryName := "myreg"
	setupRegistry(t, tempDir, registryName)

	// setup package
	packageName := "mypkg"
	packageVersions := []string{"v1.0.0", "v2.0.0"}
	packageDir, packageGitURL := setupPackageWithGit(t, tempDir, packageName, packageVersions[0])
	releasePackage(t, packageDir, packageVersions[0])
	releasePackage(t, packageDir, packageVersions[1])
	addPackageToRegistry(t, tempDir, registryName, packageGitURL)

	// Initialize project and add both dependencies
	projectDir := initPackage(t, tempDir, "myproject")
	addDependencyToProject(t, projectDir, packageName, packageVersions[0])
	addDependencyToProject(t, projectDir, packageName, packageVersions[1])

	// Remove dependency with user prompt
	cmd := exec.Command(binaryPath, "rm", packageName)
	cmd.Dir = projectDir
	cmd.Stdin = strings.NewReader("1\n") // Select the first dependency
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to remove dependency '%s': %v\nStderr: %s", packageName, err, stderr.String())
	}

	// Verify output
	expectedOutput := fmt.Sprintf("Removed dependency '%s' from project\n", packageName)
	if !strings.Contains(stdout.String(), expectedOutput) {
		t.Errorf("Expected output containing %q, got %q\nStderr: %s", expectedOutput, stdout.String(), stderr.String())
	}

	// Verify only one dependency remains
	project := loadProjectFile(t, filepath.Join(projectDir, "Project.json"))
	remainingDeps := 0
	for _, dep := range project.Deps {
		if dep.Name == packageName {
			remainingDeps++
			if dep.Version != packageVersions[1] {
				t.Errorf("Expected remaining dependency version %s, got %s", packageVersions[1], dep.Version)
			}
		}
	}
	if remainingDeps != 1 {
		t.Errorf("Expected 1 remaining dependency for '%s', got %d: %v", packageName, remainingDeps, project.Deps)
	}
}

func TestMinimalVersionSelectionBuildList(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()
	registryName := "myreg"
	setupRegistry(t, tempDir, registryName)

	// Package E
	packageDir, gitURL := setupPackageWithGit(t, tempDir, "E", "v1.1.0")
	releasePackage(t, packageDir, "v1.1.0")
	releasePackage(t, packageDir, "v1.2.0")
	releasePackage(t, packageDir, "v1.3.0")
	addPackageToRegistry(t, tempDir, registryName, gitURL)

	// Package G
	packageDir, gitURL = setupPackageWithGit(t, tempDir, "G", "v1.1.0")
	releasePackage(t, packageDir, "v1.1.0")
	addPackageToRegistry(t, tempDir, registryName, gitURL)

	// Package F
	packageDir, gitURL = setupPackageWithGit(t, tempDir, "F", "v1.1.0")
	addDependencyToProject(t, packageDir, "G", "v1.1.0")
	commitAndPushPackageChanges(t, packageDir, "added G@v1.1.0")
	releasePackage(t, packageDir, "v1.1.0")
	addPackageToRegistry(t, tempDir, registryName, gitURL)

	// Package D
	packageDir, gitURL = setupPackageWithGit(t, tempDir, "D", "v1.1.0")
	addDependencyToProject(t, packageDir, "E", "v1.1.0")
	commitAndPushPackageChanges(t, packageDir, "added E@v1.1.0")
	releasePackage(t, packageDir, "v1.1.0")
	releasePackage(t, packageDir, "v1.2.0")
	removeDependencyFromProject(t, packageDir, "E")
	addDependencyToProject(t, packageDir, "E", "v1.2.0")
	commitAndPushPackageChanges(t, packageDir, "added E@v1.2.0")
	releasePackage(t, packageDir, "v1.3.0")
	releasePackage(t, packageDir, "v1.4.0")
	addPackageToRegistry(t, tempDir, registryName, gitURL)

	// Package B
	packageDir, gitURL = setupPackageWithGit(t, tempDir, "B", "v1.1.0")
	addDependencyToProject(t, packageDir, "D", "v1.1.0")
	commitAndPushPackageChanges(t, packageDir, "added D@v1.1.0")
	releasePackage(t, packageDir, "v1.1.0")
	removeDependencyFromProject(t, packageDir, "D")
	addDependencyToProject(t, packageDir, "D", "v1.3.0")
	commitAndPushPackageChanges(t, packageDir, "added D@v1.3.0")
	releasePackage(t, packageDir, "v1.2.0")
	addPackageToRegistry(t, tempDir, registryName, gitURL)

	// Package C
	packageDir, gitURL = setupPackageWithGit(t, tempDir, "C", "v1.1.0")
	releasePackage(t, packageDir, "v1.1.0")
	addDependencyToProject(t, packageDir, "D", "v1.4.0")
	commitAndPushPackageChanges(t, packageDir, "added D@v1.4.0")
	releasePackage(t, packageDir, "v1.2.0")
	addDependencyToProject(t, packageDir, "F", "v1.1.0")
	commitAndPushPackageChanges(t, packageDir, "added F@v1.1.0")
	releasePackage(t, packageDir, "v1.3.0")
	addPackageToRegistry(t, tempDir, registryName, gitURL)

	// Package A
	packageDir, gitURL = setupPackageWithGit(t, tempDir, "A", "v1.0.0")
	addDependencyToProject(t, packageDir, "B", "v1.2.0")
	addDependencyToProject(t, packageDir, "C", "v1.2.0")

	// Run cosm activate
	stdout, stderr, err := runCommand(t, packageDir, "activate")
	if err != nil {
		t.Errorf("Unexpected error: %v\nStderr: %s", err, stderr)
	}
	expectedOutput := fmt.Sprintf("Generated build list for %s in .cosm/buildlist.json\n", "A")
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}

	// Verify build list
	buildListFile := filepath.Join(packageDir, ".cosm", "buildlist.json")
	buildList := loadBuildList(t, buildListFile)

	// Define expected dependencies
	expectedDeps := map[string]string{
		"B": "v1.2.0",
		"C": "v1.2.0",
		"D": "v1.4.0",
		"E": "v1.2.0",
	}

	// Verify the number of dependencies
	if len(buildList.Dependencies) != len(expectedDeps) {
		t.Errorf("Expected %d dependencies, got %d: %v", len(expectedDeps), len(buildList.Dependencies), buildList.Dependencies)
	}

	// Verify each expected dependency
	for name, expectedVersion := range expectedDeps {
		found := false
		for key, dep := range buildList.Dependencies {
			if dep.Name == name && dep.Version == expectedVersion {
				found = true
				// Verify other fields
				if dep.UUID == "" {
					t.Errorf("Dependency %s@%s has empty UUID in key %s", name, expectedVersion, key)
				}
				if dep.GitURL == "" {
					t.Errorf("Dependency %s@%s has empty GitURL in key %s", name, expectedVersion, key)
				}
				if dep.SHA1 == "" {
					t.Errorf("Dependency %s@%s has empty SHA1 in key %s", name, expectedVersion, key)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected dependency %s:%s not found in build list", name, expectedVersion)
		}
	}
}

func TestMakePackageAvailable(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Setup registry and package
	registryName := "myreg"
	packageName := "mypkg"
	packageVersion := "v0.1.0"
	packageDir, packageGitURL := setupPackageWithGit(t, tempDir, packageName, packageVersion)
	setupRegistry(t, tempDir, registryName)
	releasePackage(t, packageDir, packageVersion)
	addPackageToRegistry(t, tempDir, registryName, packageGitURL)

	// Load specs to get UUID and SHA1
	specs := loadSpecs(t, tempDir, registryName, packageName, packageVersion)
	cloneDir := filepath.Join(tempDir, ".cosm", "clones", specs.UUID)
	destPath := filepath.Join(tempDir, ".cosm", "packages", packageName, specs.SHA1)

	// Capture initial branch
	initialBranch := getCloneBranch(t, cloneDir)

	tests := []struct {
		name          string
		setup         func(t *testing.T)
		expectError   bool
		verifyResults func(t *testing.T)
	}{
		{
			name: "success",
			setup: func(t *testing.T) {
				// Ensure packages directory is writable
				if err := os.MkdirAll(filepath.Join(tempDir, ".cosm", "packages"), 0755); err != nil {
					t.Fatalf("Failed to create packages dir: %v", err)
				}
			},
			expectError: false,
			verifyResults: func(t *testing.T) {
				verifyPackageDestination(t, destPath)
			},
		},
		{
			name: "error_unwritable_destination",
			setup: func(t *testing.T) {
				// Clean up and recreate packages directory as unwritable
				packagesDir := filepath.Join(tempDir, ".cosm", "packages")
				if err := os.RemoveAll(packagesDir); err != nil {
					t.Fatalf("Failed to remove packages dir: %v", err)
				}
				if err := os.MkdirAll(packagesDir, 0755); err != nil {
					t.Fatalf("Failed to recreate packages dir: %v", err)
				}
				if err := os.Chmod(packagesDir, 0500); err != nil {
					t.Fatalf("Failed to make packages dir unwritable: %v", err)
				}
				t.Cleanup(func() { os.Chmod(packagesDir, 0755) })
			},
			expectError: true,
			verifyResults: func(t *testing.T) {
				if _, err := os.Stat(destPath); !os.IsNotExist(err) {
					t.Errorf("Destination directory %s was created unexpectedly", destPath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			err := commands.MakePackageAvailable(filepath.Join(tempDir, ".cosm"), registryName, packageName, packageVersion)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
			tt.verifyResults(t)
			verifyCloneBranch(t, cloneDir, initialBranch)
		})
	}
}
