package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// createProject constructs a new Project struct
func createProject(packageName, projectUUID string, authors []string, language, version string) types.Project {
	return types.Project{
		Name:     packageName,
		UUID:     projectUUID,
		Authors:  authors,
		Language: language,
		Version:  version,
		Deps:     make(map[string]string),
	}
}

// selectPackageFromResults handles the selection of a package from multiple matches
func selectPackageFromResults(packageName, versionTag string, foundPackages []types.PackageLocation) (types.PackageLocation, error) {
	if len(foundPackages) == 0 {
		return types.PackageLocation{}, fmt.Errorf("package '%s' with version '%s' not found in any registry", packageName, versionTag)
	}
	if len(foundPackages) == 1 {
		return foundPackages[0], nil
	}
	return promptUserForRegistry(packageName, versionTag, foundPackages)
}

// MakePackageAvailable copies the contents of a cloned package for a specific version
// from ~/.cosm/clones/<UUID> to ~/.cosm/packages/<packageName>/<SHA1>, excluding Git-related files,
// and ensures the clone is reverted to its previous state even on error.
func MakePackageAvailable(cosmDir, registryName, packageName, versionTag string) error {
	// Construct paths
	registriesDir := filepath.Join(cosmDir, "registries")
	specsFile := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName, versionTag, "specs.json")

	// Load specs to get UUID and SHA1
	data, err := os.ReadFile(specsFile)
	if err != nil {
		return fmt.Errorf("failed to read specs.json for %s@%s in registry %s: %v", packageName, versionTag, registryName, err)
	}
	var specs types.Specs
	if err := json.Unmarshal(data, &specs); err != nil {
		return fmt.Errorf("failed to parse specs.json for %s@%s: %v", packageName, versionTag, err)
	}
	if specs.Version != versionTag {
		return fmt.Errorf("mismatched version in specs.json: expected %s, got %s", versionTag, specs.Version)
	}
	if specs.SHA1 == "" {
		return fmt.Errorf("empty SHA1 in specs.json for %s@%s", packageName, versionTag)
	}

	// Locate clone directory
	clonePath := filepath.Join(cosmDir, "clones", specs.UUID)
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		return fmt.Errorf("clone directory for UUID %s not found at %s", specs.UUID, clonePath)
	}

	// Ensure clone is at the correct version
	if err := checkoutVersion(clonePath, specs.SHA1); err != nil {
		return fmt.Errorf("failed to checkout SHA1 %s for %s@%s: %v", specs.SHA1, packageName, versionTag, err)
	}

	// Create destination directory
	destPath := filepath.Join(cosmDir, "packages", packageName, specs.SHA1)
	if err := os.MkdirAll(destPath, 0755); err != nil {
		if revertErr := revertClone(clonePath); revertErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to revert clone after error: %v\n", revertErr)
		}
		return fmt.Errorf("failed to create destination directory %s: %v", destPath, err)
	}

	// Copy files, excluding Git-related ones
	err = filepath.Walk(clonePath, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory and .gitignore files
		if info.Name() == ".git" || info.Name() == ".gitignore" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Compute relative path and destination
		relPath, err := filepath.Rel(clonePath, srcPath)
		if err != nil {
			return fmt.Errorf("failed to compute relative path for %s: %v", srcPath, err)
		}
		if relPath == "." {
			return nil // Skip root directory itself
		}
		destFile := filepath.Join(destPath, relPath)

		// Handle directories
		if info.IsDir() {
			return os.MkdirAll(destFile, info.Mode())
		}

		// Copy file
		return copyFile(srcPath, destFile, info.Mode())
	})
	if err != nil {
		if revertErr := revertClone(clonePath); revertErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to revert clone after error: %v\n", revertErr)
		}
		return fmt.Errorf("failed to copy package files for %s@%s: %v", packageName, versionTag, err)
	}

	// Revert clone on success
	if err := revertClone(clonePath); err != nil {
		return fmt.Errorf("failed to revert clone for %s@%s: %v", packageName, versionTag, err)
	}

	return nil
}
