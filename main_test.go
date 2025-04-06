package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
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
	Name        string    `json:"name"`
	GitURL      string    `json:"giturl"`
	LastUpdated time.Time `json:"last_updated,omitempty"`
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
	Name        string    `json:"name"`
	GitURL      string    `json:"giturl"`
	LastUpdated time.Time `json:"last_updated,omitempty"`
}) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Failed to read registries.json: %v", err)
	}
	var registries []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
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
		if !exp.LastUpdated.IsZero() && got.LastUpdated.IsZero() {
			t.Errorf("Expected registry %d to have a LastUpdated time, got zero", i)
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

func TestRegistryStatus(t *testing.T) {
	tempDir := t.TempDir()
	stdout, _, err := runCommand(t, tempDir, "registry", "status", "cosmic-hub")
	checkOutput(t, stdout, "", "Status for registry 'cosmic-hub':\n  Available packages:\n    - cosmic-hub-pkg1 (v1.0.0)\n    - cosmic-hub-pkg2 (v2.1.3)\n  Last updated: 2025-04-05\n", err, false, 0)
}

func TestRegistryStatusInvalid(t *testing.T) {
	tempDir := t.TempDir()
	stdout, _, err := runCommand(t, tempDir, "registry", "status", "invalid-reg")
	checkOutput(t, stdout, "", "Error: 'invalid-reg' is not a valid registry name. Valid options: [cosmic-hub local]\n", err, true, 1)
}

func TestRegistryInit(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"
	gitURL := "https://git.example.com"
	stdout, _, err := runCommand(t, tempDir, "registry", "init", registryName, gitURL)
	checkOutput(t, stdout, "", fmt.Sprintf("Initialized registry '%s' with Git URL: %s\n", registryName, gitURL), err, false, 0)

	checkRegistriesFile(t, filepath.Join(tempDir, ".cosm", "registries.json"), []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{{Name: registryName, GitURL: gitURL}})
}

func TestRegistryInitDuplicate(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"
	gitURL := "https://git.example.com"
	registriesFile := setupRegistriesFile(t, tempDir, []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{{Name: registryName, GitURL: gitURL}})
	dataBefore, _ := os.ReadFile(registriesFile)

	stdout, _, err := runCommand(t, tempDir, "registry", "init", registryName, gitURL)
	checkOutput(t, stdout, "", fmt.Sprintf("Error: Registry '%s' already exists\n", registryName), err, true, 1)

	dataAfter, _ := os.ReadFile(registriesFile)
	if !bytes.Equal(dataBefore, dataAfter) {
		t.Errorf("registries.json changed unexpectedly")
	}
}

func TestRegistryClone(t *testing.T) {
	tempDir := t.TempDir()
	gitURL := "https://git.example.com/myreg.git"
	expectedName := "myreg.git"

	stdout, _, err := runCommand(t, tempDir, "registry", "clone", gitURL)
	checkOutput(t, stdout, "", fmt.Sprintf("Cloned registry '%s' from %s\n", expectedName, gitURL), err, false, 0)

	checkRegistriesFile(t, filepath.Join(tempDir, ".cosm", "registries.json"), []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{{Name: expectedName, GitURL: gitURL}})
}

func TestRegistryCloneDuplicate(t *testing.T) {
	tempDir := t.TempDir()
	gitURL := "https://git.example.com/myreg.git"
	registryName := "myreg.git"
	registriesFile := setupRegistriesFile(t, tempDir, []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{{Name: registryName, GitURL: gitURL}})
	dataBefore, _ := os.ReadFile(registriesFile)

	stdout, _, err := runCommand(t, tempDir, "registry", "clone", gitURL)
	checkOutput(t, stdout, "", fmt.Sprintf("Error: Registry '%s' already exists\n", registryName), err, true, 1)

	dataAfter, _ := os.ReadFile(registriesFile)
	if !bytes.Equal(dataBefore, dataAfter) {
		t.Errorf("registries.json changed unexpectedly")
	}
}

func TestRegistryDelete(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"
	gitURL := "https://git.example.com"
	registriesFile := setupRegistriesFile(t, tempDir, []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{{Name: registryName, GitURL: gitURL}})

	stdout, _, err := runCommand(t, tempDir, "registry", "delete", registryName)
	checkOutput(t, stdout, "", fmt.Sprintf("Deleted registry '%s'\n", registryName), err, false, 0)

	checkRegistriesFile(t, registriesFile, []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{})
}

func TestRegistryDeleteForce(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"
	gitURL := "https://git.example.com"
	registriesFile := setupRegistriesFile(t, tempDir, []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{{Name: registryName, GitURL: gitURL}})

	stdout, _, err := runCommand(t, tempDir, "registry", "delete", registryName, "--force")
	checkOutput(t, stdout, "", fmt.Sprintf("Force deleted registry '%s'\n", registryName), err, false, 0)

	checkRegistriesFile(t, registriesFile, []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{})
}

func TestRegistryDeleteNotFound(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"
	gitURL := "https://git.example.com"
	registriesFile := setupRegistriesFile(t, tempDir, []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{{Name: "otherreg", GitURL: gitURL}})
	dataBefore, _ := os.ReadFile(registriesFile)

	stdout, _, err := runCommand(t, tempDir, "registry", "delete", registryName)
	checkOutput(t, stdout, "", fmt.Sprintf("Error: Registry '%s' not found\n", registryName), err, true, 1)

	dataAfter, _ := os.ReadFile(registriesFile)
	if !bytes.Equal(dataBefore, dataAfter) {
		t.Errorf("registries.json changed unexpectedly")
	}
}

func TestRegistryUpdate(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"
	gitURL := "https://git.example.com"
	registriesFile := setupRegistriesFile(t, tempDir, []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{{Name: registryName, GitURL: gitURL}})

	stdout, _, err := runCommand(t, tempDir, "registry", "update", registryName)
	checkOutput(t, stdout, "", fmt.Sprintf("Updated registry '%s'\n", registryName), err, false, 0)

	// Check that LastUpdated is set
	data, err := os.ReadFile(registriesFile)
	if err != nil {
		t.Fatalf("Failed to read registries.json: %v", err)
	}
	var registries []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}
	if err := json.Unmarshal(data, &registries); err != nil {
		t.Fatalf("Failed to parse registries.json: %v", err)
	}
	if len(registries) != 1 || registries[0].Name != registryName || registries[0].GitURL != gitURL {
		t.Errorf("Registry data corrupted: %+v", registries)
	}
	if registries[0].LastUpdated.IsZero() {
		t.Errorf("Expected LastUpdated to be set, got zero")
	}
}

func TestRegistryUpdateNotFound(t *testing.T) {
	tempDir := t.TempDir()
	registryName := "myreg"
	gitURL := "https://git.example.com"
	registriesFile := setupRegistriesFile(t, tempDir, []struct {
		Name        string    `json:"name"`
		GitURL      string    `json:"giturl"`
		LastUpdated time.Time `json:"last_updated,omitempty"`
	}{{Name: "otherreg", GitURL: gitURL}})
	dataBefore, _ := os.ReadFile(registriesFile)

	stdout, _, err := runCommand(t, tempDir, "registry", "update", registryName)
	checkOutput(t, stdout, "", fmt.Sprintf("Error: Registry '%s' not found\n", registryName), err, true, 1)

	dataAfter, _ := os.ReadFile(registriesFile)
	if !bytes.Equal(dataBefore, dataAfter) {
		t.Errorf("registries.json changed unexpectedly")
	}
}
