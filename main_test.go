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

func TestRegistryAdd(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	tests := []struct {
		name           string
		registryName   string
		packageName    string
		packageVersion string
		setup          func(t *testing.T, registryDir string) (string, string) // Returns packageDir, packageGitURL
		input          string                                                  // Stdin input for registry selection
		args           []string                                                // Command arguments
		expectError    bool
		expectedStderr string
		verifyResults  func(t *testing.T, registryDir, packageDir, packageGitURL string)
	}{
		{
			name:           "success",
			registryName:   "myreg1",
			packageName:    "mypkg",
			packageVersion: "v0.1.0",
			setup: func(t *testing.T, registryDir string) (string, string) {
				packageDir, gitURL := setupPackageWithGit(t, tempDir, "mypkg", "v0.1.0")
				tagPackageVersion(t, packageDir, "v0.1.0")
				return packageDir, gitURL
			},
			input: "\n",                                      // Default registry selection
			args:  []string{"registry", "add", "myreg1", ""}, // gitURL set in loop
			verifyResults: func(t *testing.T, registryDir, packageDir, packageGitURL string) {
				project := loadProjectFile(t, filepath.Join(packageDir, "Project.json"))
				verifyRegistryPackage(t, registryDir, "mypkg", project.UUID, "v0.1.0", packageGitURL)
				verifyPackageCloneExists(t, tempDir, project.UUID)
			},
		},
		{
			name:           "error_duplicate_package",
			registryName:   "myreg2",
			packageName:    "mypkg2",
			packageVersion: "v0.1.0",
			setup: func(t *testing.T, registryDir string) (string, string) {
				packageDir, gitURL := setupPackageWithGit(t, tempDir, "mypkg2", "v0.1.0")
				tagPackageVersion(t, packageDir, "v0.1.0")
				addPackageToRegistry(t, tempDir, filepath.Base(registryDir), gitURL)
				return packageDir, gitURL
			},
			input:          "\n",
			args:           []string{"registry", "add", "myreg2", ""}, // gitURL set in loop
			expectError:    true,
			expectedStderr: "Error: package 'mypkg2' is already registered in registry 'myreg2'\n",
			verifyResults: func(t *testing.T, registryDir, packageDir, packageGitURL string) {
				// No additional verification needed; error case should not modify registry
			},
		},
		{
			name:           "error_invalid_git_url",
			registryName:   "myreg3",
			packageName:    "invalidpkg",
			packageVersion: "v0.1.0",
			setup: func(t *testing.T, registryDir string) (string, string) {
				return "", "file:///nonexistent.git"
			},
			input:          "\n",
			args:           []string{"registry", "add", "myreg3", "file:///nonexistent.git"},
			expectError:    true,
			expectedStderr: "Error: failed to clone package repository at 'file:///nonexistent.git'",
			verifyResults: func(t *testing.T, registryDir, packageDir, packageGitURL string) {
				// Verify registry remains unchanged
				checkRegistryMetaFile(t, filepath.Join(registryDir, "registry.json"), types.Registry{
					Name:     "myreg3",
					Packages: make(map[string]string),
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup registry for this subtest
			_, registryDir := setupRegistry(t, tempDir, tt.registryName)

			packageDir, packageGitURL := tt.setup(t, registryDir)
			args := tt.args
			if packageGitURL != "" && args[len(args)-1] == "" {
				args[len(args)-1] = packageGitURL
			}

			cmd := exec.Command(binaryPath, args...)
			cmd.Dir = tempDir
			cmd.Stdin = strings.NewReader(tt.input)
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
				expectedOutput := fmt.Sprintf("Added package '%s' with version '%s' to registry '%s'\n", tt.packageName, tt.packageVersion, tt.registryName)
				if stdout.String() != expectedOutput {
					t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout.String(), stderr.String())
				}
			}

			tt.verifyResults(t, registryDir, packageDir, packageGitURL)
		})
	}
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

	// Verify results
	expectedOutput := fmt.Sprintf("Cloned registry '%s' from %s\n", registryName, gitURL)
	if stdout.String() != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout.String(), stderr.String())
	}
	project := loadProjectFile(t, filepath.Join(packageDir, "Project.json"))
	verifyRegistryCloned(t, tempDir, registryDir, registryName, map[string]string{packageName: project.UUID})
	checkRegistriesFile(t, filepath.Join(tempDir, ".cosm", "registries", "registries.json"), []string{registryName})
	verifyRegistryPackage(t, registryDir, packageName, project.UUID, version, packageGitURL)
}

func TestRelease(t *testing.T) {
	tempDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create registry
	registryName := "myreg"
	_, registryDir := setupRegistry(t, tempDir, registryName)

	// Create a package and add to registry
	packageName := "mypkg"
	version := "v1.2.3"
	packageDir, gitURL := setupPackageWithGit(t, tempDir, packageName, version)
	addPackageToRegistry(t, tempDir, registryName, gitURL)

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
