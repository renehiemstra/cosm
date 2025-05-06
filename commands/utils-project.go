package commands

import (
	"cosm/types"
	"fmt"
	"os"
	"path/filepath"
)

// createProject constructs a new Project struct
func createProject(packageName, projectUUID string, authors []string, language, version string) types.Project {
	return types.Project{
		Name:     packageName,
		UUID:     projectUUID,
		Authors:  authors,
		Language: language,
		Version:  version,
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
func MakePackageAvailable(cosmDir string, specs *types.Specs) error {
	if err := validateSpecs(specs); err != nil {
		return err
	}

	destPath := filepath.Join(cosmDir, "packages", specs.Name, specs.SHA1)
	if checkDestinationExists(destPath) {
		return nil
	}

	clonePath := filepath.Join(cosmDir, "clones", specs.UUID)
	if err := prepareClone(clonePath, specs.SHA1); err != nil {
		return fmt.Errorf("failed to prepare clone for %s@%s: %v", specs.Name, specs.Version, err)
	}

	if err := copyPackageFiles(clonePath, destPath); err != nil {
		if revertErr := revertClone(clonePath); revertErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to revert clone after error: %v\n", revertErr)
		}
		return fmt.Errorf("failed to copy package files for %s@%s: %v", specs.Name, specs.Version, err)
	}

	if err := revertClone(clonePath); err != nil {
		return fmt.Errorf("failed to revert clone for %s@%s: %v", specs.Name, specs.Version, err)
	}

	return nil
}

// validateSpecs ensures the Specs object has valid fields
func validateSpecs(specs *types.Specs) error {
	if specs.UUID == "" {
		return fmt.Errorf("empty UUID in specs")
	}
	if specs.SHA1 == "" {
		return fmt.Errorf("empty SHA1 in specs")
	}
	if specs.Version == "" {
		return fmt.Errorf("empty version in specs")
	}
	if specs.Name == "" {
		return fmt.Errorf("empty package name in specs")
	}
	return nil
}

// checkDestinationExists checks if the destination directory exists with Project.json
func checkDestinationExists(destPath string) bool {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return false
	}
	projectFile := filepath.Join(destPath, "Project.json")
	if _, err := os.Stat(projectFile); os.IsNotExist(err) {
		return false
	}
	return true
}

// prepareClone verifies the clone directory exists and checks out the specified SHA1
func prepareClone(clonePath, sha1 string) error {
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		return fmt.Errorf("clone directory not found at %s", clonePath)
	}
	if err := checkoutVersion(clonePath, sha1); err != nil {
		return fmt.Errorf("failed to checkout SHA1 %s: %v", sha1, err)
	}
	return nil
}

// copyPackageFiles creates the destination directory and copies files, excluding Git-related ones
func copyPackageFiles(clonePath, destPath string) error {
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", destPath, err)
	}

	return filepath.Walk(clonePath, func(srcPath string, info os.FileInfo, err error) error {
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
}
