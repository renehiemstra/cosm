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

// setupRegistry initializes a registry with a bare Git remote using cosm registry init
func setupRegistry(t *testing.T, tempDir, registryName string) (string, string) {
	gitURL := createBareRepo(t, tempDir, registryName+".git")
	_, _, err := runCommand(t, tempDir, "registry", "init", registryName, gitURL)
	if err != nil {
		t.Fatalf("Failed to init registry '%s' with git-url '%s': %v", registryName, gitURL, err)
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
	// Verify Project.json exists
	projectFile := filepath.Join(packageDir, "Project.json")
	if _, err := os.Stat(projectFile); os.IsNotExist(err) {
		t.Fatalf("Project.json not found in %s", packageDir)
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
	if err := exec.Command("git", "branch", "-m", "main").Run(); err != nil {
		t.Fatalf("Failed to set main branch for %s: %v", packageName, err)
	}
	bareRepoURL := createBareRepo(t, tempDir, packageName+".git")
	if err := exec.Command("git", "remote", "add", "origin", bareRepoURL).Run(); err != nil {
		t.Fatalf("Failed to add remote for %s: %v", packageName, err)
	}
	cmd := exec.Command("git", "push", "origin", "main")
	cmd.Dir = packageDir
	_, err := cmd.CombinedOutput()
	if err != nil {
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
	args := []string{"add", packageName}
	if version != "" {
		args = append(args, version)
	}
	cmd := exec.Command(binaryPath, args...)
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

// removeDependencyFromProject removes a dependency from a project using cosm rm and verifies the output
func removeDependencyFromProject(t *testing.T, projectDir, packageName string) (stdout, stderr string) {
	t.Helper()
	stdout, stderr, err := runCommand(t, projectDir, "rm", packageName)
	expectedOutput := fmt.Sprintf("Removed dependency '%s' from project\n", packageName)
	if err != nil {
		t.Errorf("Failed to remove dependency '%s': %v\nStderr: %s", packageName, err, stderr)
	}
	if stdout != expectedOutput {
		t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
	}
	return stdout, stderr
}

// setupTestEnv sets up a temporary environment with a Git config
func setupTestEnv(t *testing.T) (tempDir string, cleanup func()) {
	tempDir = t.TempDir()
	os.Setenv("HOME", tempDir)
	_ = setupTempGitConfig(t, tempDir)
	os.Setenv("COSM_DEPOT_PATH", filepath.Join(tempDir, ".cosm"))
	commands.InitializeCosm()
	cleanup = func() { os.Unsetenv("HOME"); os.Unsetenv("COSM_DEPOT_PATH") }
	return tempDir, cleanup
}

// createBareRepo creates a bare Git repository and returns its file:// URL
func createBareRepo(t *testing.T, dir string, name string) string {
	t.Helper()
	bareRepoPath := filepath.Join(dir, name)
	if err := os.Mkdir(bareRepoPath, 0755); err != nil {
		t.Fatalf("Failed to create package dir %s: %v", bareRepoPath, err)
	}
	if _, err := commands.GitCommand(bareRepoPath, "init", "--bare"); err != nil {
		t.Fatalf("Failed to init bare Git repo in %s: %v", bareRepoPath, err)
	}
	if _, err := commands.GitCommand(bareRepoPath, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
		t.Fatalf("Failed to set HEAD in bare repo %s: %v", bareRepoPath, err)
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
		project.Deps = make(map[string]types.Dependency)
	}
	return project
}

// removeFromRegistry executes the cosm registry rm command and verifies its output
func removeFromRegistry(t *testing.T, dir, registryName, packageName string, version string) (stdout, stderr string) {
	t.Helper()
	if version == "" {
		stdout, stderr, err := runCommand(t, dir, "registry", "rm", registryName, packageName, "--force")
		expectedOutput := fmt.Sprintf("Removed package '%s' from registry '%s'\n", packageName, registryName)
		if err != nil {
			t.Errorf("Failed to remove package %s: %v\nStderr: %s", packageName, err, stderr)
		}
		if stdout != expectedOutput {
			t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
		}
		return stdout, stderr
	} else {
		stdout, stderr, err := runCommand(t, dir, "registry", "rm", registryName, packageName, version, "--force")
		expectedOutput := fmt.Sprintf("Removed version '%s' of package '%s' from registry '%s'\n", version, packageName, registryName)
		if err != nil {
			t.Errorf("Failed to remove version %s: %v\nStderr: %s", version, err, stderr)
		}
		if stdout != expectedOutput {
			t.Errorf("Expected output %q, got %q\nStderr: %s", expectedOutput, stdout, stderr)
		}
		return stdout, stderr
	}
}

// commitAndPushPackageChanges commits and pushes changes in the package directory
func commitAndPushPackageChanges(t *testing.T, packageDir, commitMessage string) {
	t.Helper()
	// Stage all changes
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = packageDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to stage changes in %s: %v", packageDir, err)
	}

	// Commit changes
	cmd = exec.Command("git", "commit", "-m", commitMessage)
	cmd.Dir = packageDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit changes in %s: %v", packageDir, err)
	}

	// Push to origin main
	cmd = exec.Command("git", "push", "origin", "main")
	cmd.Dir = packageDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Git push failed in %s: %v\nOutput:\n%s", packageDir, err, string(output))
		t.Fatalf("Failed to push main for %s: %v", filepath.Base(packageDir), err)
	}
}

// loadBuildList reads and parses buildlist.json from a given file path
func loadBuildList(t *testing.T, buildListFile string) types.BuildList {
	t.Helper()
	data, err := os.ReadFile(buildListFile)
	if err != nil {
		t.Fatalf("Failed to read buildlist.json: %v", err)
	}
	var buildList types.BuildList
	if err := json.Unmarshal(data, &buildList); err != nil {
		t.Fatalf("Failed to parse buildlist.json: %v", err)
	}
	if buildList.Dependencies == nil {
		buildList.Dependencies = make(map[string]types.BuildListDependency)
	}
	return buildList
}

/////////////////////// Check HELPER FUNCTIONS ///////////////////////

// verifyProjectDependencies checks the Project.json dependencies
func verifyProjectDependencies(t *testing.T, projectFile, packageName, expectedVersion string) {
	project := loadProjectFile(t, projectFile)
	found := false
	for key, dep := range project.Deps {
		if dep.Name == packageName && dep.Version == expectedVersion {
			// Verify key format: <uuid>@<major version>
			parts := strings.Split(key, "@")
			if len(parts) != 2 {
				t.Errorf("Expected dependency key format <uuid>@<major version>, got %q", key)
			}
			if _, err := uuid.Parse(parts[0]); err != nil {
				t.Errorf("Expected valid UUID in key %q, got error: %v", key, err)
			}
			majorVersion, err := commands.GetMajorVersion(expectedVersion)
			if err != nil || parts[1] != majorVersion {
				t.Errorf("Expected major version %q in key %q, got %q", majorVersion, key, parts[1])
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected dependency %s:%s, got %v", packageName, expectedVersion, project.Deps)
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
	for pkgName, expectedInfo := range expected.Packages {
		gotInfo, exists := registry.Packages[pkgName]
		if !exists {
			t.Errorf("Expected package %q in registry, not found", pkgName)
		} else {
			if gotInfo.UUID != expectedInfo.UUID {
				t.Errorf("Expected UUID %q for package %q, got %q", expectedInfo.UUID, pkgName, gotInfo.UUID)
			}
			if gotInfo.GitURL != expectedInfo.GitURL {
				t.Errorf("Expected GitURL %q for package %q, got %q", expectedInfo.GitURL, pkgName, gotInfo.GitURL)
			}
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
		for depKey, expDep := range expected.Deps {
			gotDep, exists := project.Deps[depKey]
			if !exists || gotDep.Version != expDep.Version {
				t.Errorf("Expected dep %q: %q, got %q", expDep.Name, expDep.Version, gotDep.Version)
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

// loadSpecs reads and parses specs.json for a package version
func loadSpecs(t *testing.T, tempDir, registryName, packageName, version string) types.Specs {
	t.Helper()
	specsFile := filepath.Join(tempDir, ".cosm", "registries", registryName, strings.ToUpper(string(packageName[0])), packageName, version, "specs.json")
	data, err := os.ReadFile(specsFile)
	if err != nil {
		t.Fatalf("Failed to read specs.json: %v", err)
	}
	var specs types.Specs
	if err := json.Unmarshal(data, &specs); err != nil {
		t.Fatalf("Failed to parse specs.json: %v", err)
	}
	return specs
}

// getCloneBranch retrieves the current branch of a Git clone
func getCloneBranch(t *testing.T, cloneDir string) string {
	t.Helper()
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(cloneDir); err != nil {
		t.Fatalf("Failed to change to clone dir %s: %v", cloneDir, err)
	}
	defer os.Chdir(currentDir)
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get branch: %v", err)
	}
	return strings.TrimSpace(string(output))
}

// verifyCloneBranch checks if the clone is on the expected branch
func verifyCloneBranch(t *testing.T, cloneDir, expectedBranch string) {
	t.Helper()
	currentBranch := getCloneBranch(t, cloneDir)
	if currentBranch != expectedBranch {
		t.Errorf("Expected clone branch %q, got %q", expectedBranch, currentBranch)
	}
}

// verifyPackageDestination checks if the destination directory exists and contains expected files
func verifyPackageDestination(t *testing.T, destPath string) {
	t.Helper()
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Errorf("Destination directory %s not created", destPath)
	}
	// Check for Project.json as a basic verification of copied content
	projectFile := filepath.Join(destPath, "Project.json")
	if _, err := os.Stat(projectFile); os.IsNotExist(err) {
		t.Errorf("Expected Project.json in %s, not found", destPath)
	}
}

// tagPackageVersion tags a specific version in the package's Git repository
func tagPackageVersion(t *testing.T, packageDir, version string) {
	t.Helper()
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(packageDir); err != nil {
		t.Fatalf("Failed to change to package dir %s: %v", packageDir, err)
	}
	defer os.Chdir(currentDir)

	if err := exec.Command("git", "tag", version).Run(); err != nil {
		t.Fatalf("Failed to tag version %s: %v", version, err)
	}
	if err := exec.Command("git", "push", "origin", version).Run(); err != nil {
		t.Fatalf("Failed to push tag %s: %v", version, err)
	}
}

// verifyPackageInRegistry verifies that a package is present in registry.json with the correct UUID and GitURL
func verifyPackageInRegistry(t *testing.T, registryDir, packageName, packageUUID, packageGitURL string) {
	t.Helper()
	registryFile := filepath.Join(registryDir, "registry.json")
	data, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatalf("Failed to read registry.json: %v", err)
	}
	var registry types.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("Failed to parse registry.json: %v", err)
	}
	pkgInfo, exists := registry.Packages[packageName]
	if !exists {
		t.Errorf("Expected package %q in registry.json, not found", packageName)
	}
	if pkgInfo.UUID != packageUUID {
		t.Errorf("Expected UUID %q for package %q, got %q", packageUUID, packageName, pkgInfo.UUID)
	}
	if pkgInfo.GitURL != packageGitURL {
		t.Errorf("Expected GitURL %q for package %q, got %q", packageGitURL, packageName, pkgInfo.GitURL)
	}
}

// verifyRegistryPackage verifies the specs.json for a specific version of a package
func verifyRegistryPackage(t *testing.T, registryDir, packageName, packageUUID, gitURL, version string) {
	t.Helper()
	specsFile := filepath.Join(registryDir, strings.ToUpper(string(packageName[0])), packageName, version, "specs.json")
	data, err := os.ReadFile(specsFile)
	if err != nil {
		t.Fatalf("Failed to read specs.json for %s@%s: %v", packageName, version, err)
	}
	var specs types.Specs
	if err := json.Unmarshal(data, &specs); err != nil {
		t.Fatalf("Failed to parse specs.json for %s@%s: %v", packageName, version, err)
	}
	if specs.Name != packageName {
		t.Errorf("Expected specs.Name %q, got %q", packageName, specs.Name)
	}
	if specs.UUID != packageUUID {
		t.Errorf("Expected specs.UUID %q, got %q", packageUUID, specs.UUID)
	}
	if specs.Version != version {
		t.Errorf("Expected specs.Version %q, got %q", version, specs.Version)
	}
	if specs.GitURL != gitURL {
		t.Errorf("Expected specs.GitURL %q, got %q", gitURL, specs.GitURL)
	}
	if specs.SHA1 == "" {
		t.Errorf("Expected non-empty SHA1 in specs.json for %s@%s", packageName, version)
	}
}

// verifyPackageCloneExists ensures the package clone exists in the clones directory
func verifyPackageCloneExists(t *testing.T, tempDir, packageUUID string) {
	clonePath := filepath.Join(tempDir, ".cosm", "clones", packageUUID)
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		t.Errorf("Package clone not found at %s", clonePath)
	}
}

// verifyRegistryDeleted verifies that a registry was deleted and removed from registries.json
func verifyRegistryDeleted(t *testing.T, registriesDir, registryName string) {
	t.Helper()
	registryPath := filepath.Join(registriesDir, registryName)
	if _, err := os.Stat(registryPath); !os.IsNotExist(err) {
		t.Errorf("Registry directory %s was not deleted", registryPath)
	}
	data, err := os.ReadFile(filepath.Join(registriesDir, "registries.json"))
	if err != nil {
		t.Fatalf("Failed to read registries.json: %v", err)
	}
	var registryNames []string
	if err := json.Unmarshal(data, &registryNames); err != nil {
		t.Fatalf("Failed to parse registries.json: %v", err)
	}
	for _, name := range registryNames {
		if name == registryName {
			t.Errorf("Registry '%s' was not removed from registries.json", registryName)
		}
	}
}

// deleteRegistry deletes a registry programmatically
func deleteRegistry(t *testing.T, tempDir, registryName string, force bool) {
	t.Helper()
	args := []string{"registry", "delete", registryName}
	if force {
		args = append(args, "--force")
	}
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = tempDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to delete registry '%s': %v\nStderr: %s", registryName, err, stderr.String())
	}
}

// verifyRegistryCloned verifies that a registry was cloned and its metadata is correct
func verifyRegistryCloned(t *testing.T, tempDir, registryDir, registryName string, packages map[string]types.PackageInfo) {
	t.Helper()
	// Verify registry directory exists
	if _, err := os.Stat(registryDir); os.IsNotExist(err) {
		t.Errorf("Registry directory %s not created", registryDir)
	}

	// Verify registry.json
	checkRegistryMetaFile(t, filepath.Join(registryDir, "registry.json"), types.Registry{
		Name:     registryName,
		Packages: packages,
	})
}

func verifyPackageRemoved(t *testing.T, registryDir, packageName, version string) {
	t.Helper()
	// Check registry.json
	registry, _, err := commands.LoadRegistryMetadata(filepath.Dir(registryDir), filepath.Base(registryDir))
	if err != nil {
		t.Fatalf("Failed to load registry metadata: %v", err)
	}
	if _, exists := registry.Packages[packageName]; version == "" && exists {
		t.Errorf("Package '%s' still exists in registry.json", packageName)
	}

	// Check package directory
	packageFirstLetter := strings.ToUpper(string(packageName[0]))
	packageDir := filepath.Join(registryDir, packageFirstLetter, packageName)
	if version == "" {
		if _, err := os.Stat(packageDir); !os.IsNotExist(err) {
			t.Errorf("Package directory '%s' still exists", packageDir)
		}
		return
	}

	// Check version-specific removal
	versionsFile := filepath.Join(packageDir, "versions.json")
	data, err := os.ReadFile(versionsFile)
	if err != nil {
		t.Fatalf("Failed to read versions.json: %v", err)
	}
	var versions []string
	if err := json.Unmarshal(data, &versions); err != nil {
		t.Fatalf("Failed to parse versions.json: %v", err)
	}
	for _, v := range versions {
		if v == version {
			t.Errorf("Version '%s' still exists in versions.json for package '%s'", version, packageName)
		}
	}
	versionDir := filepath.Join(packageDir, version)
	if _, err := os.Stat(versionDir); !os.IsNotExist(err) {
		t.Errorf("Version directory '%s' still exists for package '%s'", versionDir, packageName)
	}
}

func verifyRemoteUpdated(t *testing.T, tempDir, registryDir, expectedCommitMsg string) {
	t.Helper()
	// Change to the registry's local repository
	if err := os.Chdir(registryDir); err != nil {
		t.Fatalf("Failed to change to registry directory %s: %v", registryDir, err)
	}

	// Check the latest commit message
	logCmd := exec.Command("git", "log", "-1", "--pretty=%B")
	logOutput, err := logCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get latest commit message: %v", err)
	}
	commitMsg := strings.TrimSpace(string(logOutput))
	if commitMsg != expectedCommitMsg {
		t.Errorf("Expected commit message %q, got %q", expectedCommitMsg, commitMsg)
	}
}
