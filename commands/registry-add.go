package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// RegistryAdd adds a package with all versions or a specific version to a registry
func RegistryAdd(cmd *cobra.Command, args []string) error {
	registryName, packageName, versionTag, packageGitURL, cosmDir, registriesDir, err := parseArgsAndSetup(args)
	if err != nil {
		return err
	}
	if err := updateSingleRegistry(registriesDir, registryName); err != nil {
		return err
	}
	registry, registryMetaFile, err := LoadRegistryMetadata(registriesDir, registryName)
	if err != nil {
		return err
	}

	var packageUUID string
	var tags []string
	var tmpClonePath, currentDir string
	if len(args) == 2 {
		// Mode 1: Add package with all versions
		tmpClonePath, err = clonePackageToTempDir(cosmDir, packageGitURL)
		if err != nil {
			return err
		}
		currentDir, err = os.Getwd()
		if err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return fmt.Errorf("failed to get current directory: %v", err)
		}
		if err := enterCloneDir(tmpClonePath); err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return err
		}
		// Fetch tags to ensure latest tags are available
		fetchCmd := exec.Command("git", "fetch", "--tags")
		if fetchOutput, err := fetchCmd.CombinedOutput(); err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return fmt.Errorf("failed to fetch tags for repository at '%s': %v\nOutput: %s", packageGitURL, err, fetchOutput)
		}
		// Validate Project.json to get package name and UUID
		project, err := loadProjectFile(tmpClonePath)
		if err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return err
		}
		err = validateProject(project)
		if err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return err
		}
		packageName = project.Name
		packageUUID = project.UUID
		if err := ensurePackageNotRegistered(registry, packageName, registryName, tmpClonePath); err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return err
		}
		tags, err = validateAndCollectVersionTags(packageGitURL, "")
		if err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return err
		}
		packageDir, err := setupPackageDir(registriesDir, registryName, packageName)
		if err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return err
		}
		if len(tags) > 0 {
			// Update versions for all tags, handling checkout
			if err := updatePackageVersions(packageDir, packageName, packageUUID, packageGitURL, tags, registriesDir, tmpClonePath); err != nil {
				cleanupRegistryAdd(currentDir, tmpClonePath)
				return err
			}
		}

		// Update registry.json and move clone
		registry.Packages[packageName] = types.PackageInfo{
			UUID:   packageUUID,
			GitURL: packageGitURL,
		}
		data, err := json.MarshalIndent(registry, "", "  ")
		if err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return fmt.Errorf("failed to marshal registry.json for '%s': %v", registryName, err)
		}
		if err := os.WriteFile(registryMetaFile, data, 0644); err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return fmt.Errorf("failed to write registry.json for '%s': %v", registryName, err)
		}
		_, err = moveCloneToPermanentDir(cosmDir, tmpClonePath, packageUUID)
		if err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return err
		}
		commitMsg := fmt.Sprintf("Added package %s", packageName)
		if len(tags) > 0 {
			commitMsg = fmt.Sprintf("Added package %s version %s", packageName, tags[0])
		}
		if err := commitAndPushRegistryChanges(registriesDir, registryName, commitMsg); err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return err
		}
		if err := restoreRegistryAddDir(currentDir); err != nil {
			cleanupRegistryAdd(currentDir, tmpClonePath)
			return err
		}
		cleanupRegistryAdd(currentDir, tmpClonePath)
		fmt.Printf("Added package '%s' to registry '%s'\n", packageName, registryName)
		return nil
	}

	// Mode 2: Add specific version of existing package
	pkgInfo, exists := registry.Packages[packageName]
	if !exists {
		return fmt.Errorf("package '%s' not found in registry '%s'", packageName, registryName)
	}
	packageUUID = pkgInfo.UUID
	packageGitURL = pkgInfo.GitURL

	// Check if version is already registered
	packageDir := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName)
	versionsFile := filepath.Join(packageDir, "versions.json")
	var existingVersions []string
	if data, err := os.ReadFile(versionsFile); err == nil {
		if err := json.Unmarshal(data, &existingVersions); err != nil {
			return fmt.Errorf("failed to parse versions.json for package '%s': %v", packageName, err)
		}
		if contains(existingVersions, versionTag) {
			return fmt.Errorf("version '%s' of package '%s' is already registered in registry '%s'", versionTag, packageName, registryName)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read versions.json for package '%s': %v", packageName, err)
	}

	// Check if package is cloned
	clonePath := filepath.Join(cosmDir, "clones", packageUUID)
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		tmpClonePath, err := clonePackageToTempDir(cosmDir, packageGitURL)
		if err != nil {
			return err
		}
		defer cleanupTempClone(tmpClonePath)
		clonePath, err = moveCloneToPermanentDir(cosmDir, tmpClonePath, packageUUID)
		if err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("failed to check clone at %s: %v", clonePath, err)
	}

	// Update versions for the specific tag, handling checkout
	if err := updatePackageVersions(packageDir, packageName, packageUUID, packageGitURL, []string{versionTag}, registriesDir, clonePath); err != nil {
		return err
	}

	// Commit and push registry changes
	commitMsg := fmt.Sprintf("Added version %s of package %s", versionTag, packageName)
	if err := commitAndPushRegistryChanges(registriesDir, registryName, commitMsg); err != nil {
		return err
	}

	fmt.Printf("Added version '%s' of package '%s' to registry '%s'\n", versionTag, packageName, registryName)
	return nil
}

// parseArgsAndSetup validates arguments and sets up directories
func parseArgsAndSetup(args []string) (registryName, packageName, versionTag, packageGitURL, cosmDir, registriesDir string, err error) {
	if len(args) != 2 && len(args) != 3 {
		return "", "", "", "", "", "", fmt.Errorf("requires two arguments (registry name, package giturl) or three arguments (registry name, package name, version)")
	}
	registryName = args[0]
	if registryName == "" {
		return "", "", "", "", "", "", fmt.Errorf("registry name must not be empty")
	}
	cosmDir, err = getCosmDir()
	if err != nil {
		return "", "", "", "", "", "", err
	}
	registriesDir = filepath.Join(cosmDir, "registries")
	if len(args) == 2 {
		packageGitURL = args[1]
		if packageGitURL == "" {
			return "", "", "", "", "", "", fmt.Errorf("package giturl must not be empty")
		}
		return registryName, "", "", packageGitURL, cosmDir, registriesDir, nil
	}
	packageName = args[1]
	versionTag = args[2]
	if packageName == "" {
		return "", "", "", "", "", "", fmt.Errorf("package name must not be empty")
	}
	if versionTag == "" || !strings.HasPrefix(versionTag, "v") {
		return "", "", "", "", "", "", fmt.Errorf("version must be non-empty and start with 'v'")
	}
	return registryName, packageName, versionTag, "", cosmDir, registriesDir, nil
}

// ensurePackageNotRegistered checks if the package is already in the registry
func ensurePackageNotRegistered(registry types.Registry, packageName, registryName, tmpClonePath string) error {
	if _, exists := registry.Packages[packageName]; exists {
		cleanupErr := cleanupTempClone(tmpClonePath)
		if cleanupErr != nil {
			return fmt.Errorf("package '%s' is already registered in registry '%s'; cleanup failed: %v", packageName, registryName, cleanupErr)
		}
		return fmt.Errorf("package '%s' is already registered in registry '%s'", packageName, registryName)
	}
	return nil
}

// moveCloneToPermanentDir moves the cloned directory to its permanent location, replacing any existing clone
func moveCloneToPermanentDir(cosmDir, tmpClonePath, packageUUID string) (string, error) {
	clonesDir := filepath.Join(cosmDir, "clones")
	packageClonePath := filepath.Join(clonesDir, packageUUID)

	// If the permanent clone directory already exists, remove it
	if _, err := os.Stat(packageClonePath); !os.IsNotExist(err) {
		if err := os.RemoveAll(packageClonePath); err != nil {
			return "", fmt.Errorf("failed to remove existing clone at %s: %v", packageClonePath, err)
		}
		fmt.Fprintf(os.Stderr, "Warning: Replaced existing clone for UUID '%s' at %s\n", packageUUID, packageClonePath)
	}

	// Move the temporary clone to the permanent location
	if err := os.Rename(tmpClonePath, packageClonePath); err != nil {
		return "", fmt.Errorf("failed to move package to %s: %v", packageClonePath, err)
	}
	return packageClonePath, nil
}

// validateAndCollectVersionTags fetches Git tags, or returns empty slice if none exist
func validateAndCollectVersionTags(packageGitURL string, packageVersion string) ([]string, error) {
	tagOutput, err := exec.Command("git", "tag").CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(tagOutput))) == 0 {
		return []string{}, nil // No tags, return empty slice
	}

	tags := strings.Split(strings.TrimSpace(string(tagOutput)), "\n")
	var validTags []string
	for _, tag := range tags {
		if strings.HasPrefix(tag, "v") && len(strings.Split(tag, ".")) >= 2 {
			validTags = append(validTags, tag)
		}
	}
	return validTags, nil
}

// setupPackageDir creates the package directory structure
func setupPackageDir(registriesDir, registryName, packageName string) (string, error) {
	packageFirstLetter := strings.ToUpper(string(packageName[0]))
	packageDir := filepath.Join(registriesDir, registryName, packageFirstLetter, packageName)
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create package directory %s: %v", packageDir, err)
	}
	return packageDir, nil
}

// updatePackageVersions updates versions.json with the specified tags
func updatePackageVersions(packageDir, packageName, packageUUID, packageGitURL string, tags []string, registriesDir, clonePath string) error {
	versionsFile := filepath.Join(packageDir, "versions.json")
	var versions []string
	if data, err := os.ReadFile(versionsFile); err == nil {
		if err := json.Unmarshal(data, &versions); err != nil {
			return fmt.Errorf("failed to parse versions.json for package '%s': %v", packageName, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read versions.json for package '%s': %v", packageName, err)
	}

	// Process each tag
	for _, tag := range tags {
		if !contains(versions, tag) {

			// Fetch latest changes from remote to ensure tag commits are available
			fetchCmd := exec.Command("git", "fetch", "origin")
			fetchCmd.Dir = clonePath
			if fetchOutput, err := fetchCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to fetch remote changes for package '%s': %v\nOutput: %s", packageName, err, fetchOutput)
			}

			// Checkout the specific version tag
			checkoutCmd := exec.Command("git", "checkout", tag)
			checkoutCmd.Dir = clonePath
			if checkoutOutput, err := checkoutCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to checkout tag '%s' for package '%s': %v\nOutput: %s", tag, packageName, err, checkoutOutput)
			}

			// Load Project.json for this tag
			project, err := loadProjectFile(clonePath)
			validateProject(project)

			// Next process possible error in validating project file
			if err != nil {
				return fmt.Errorf("failed to load Project.json for tag '%s': %v", tag, err)
			}

			// first revert back to HEAD
			if revertErr := revertClone(clonePath); revertErr != nil {
				return fmt.Errorf("failed to add package version '%s': %v; revert failed: %v", tag, err, revertErr)
			}

			sha1, err := getSHA1FromTaggedVersion(clonePath, tag)
			// error in retreiving sha1 for tagged version
			if err != nil {
				return fmt.Errorf("failed to get sha1 commit hash for tag '%s': %v", tag, err)
			}

			// Add the version using the project data for this tag
			if err := addPackageVersion(packageDir, packageName, packageUUID, packageGitURL, sha1, tag, project, registriesDir); err != nil {
				return err
			}

			versions = append(versions, tag)
		}
	}

	// Write updated versions.json
	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal versions.json for package '%s': %v", packageName, err)
	}
	if err := os.WriteFile(versionsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write versions.json for package '%s': %v", packageName, err)
	}

	return nil
}

// cleanupRegistryAdd reverts to the original directory and removes tmpClonePath
func cleanupRegistryAdd(originalDir, tmpClonePath string) {
	if err := os.Chdir(originalDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to return to original directory %s: %v\n", originalDir, err)
	}
	if tmpClonePath != "" {
		if err := os.RemoveAll(tmpClonePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clean up temporary clone directory %s: %v\n", tmpClonePath, err)
		}
	}
}

// restoreRegistryAddDir returns to the original directory
func restoreRegistryAddDir(originalDir string) error {
	if err := os.Chdir(originalDir); err != nil {
		return fmt.Errorf("failed to return to original directory %s: %v", originalDir, err)
	}
	return nil
}

// enterCloneDir changes to the temporary clone directory
func enterCloneDir(tmpClonePath string) error {
	if err := os.Chdir(tmpClonePath); err != nil {
		cleanupErr := cleanupTempClone(tmpClonePath)
		if cleanupErr != nil {
			return fmt.Errorf("failed to change to cloned directory %s: %v; cleanup failed: %v", tmpClonePath, err, cleanupErr)
		}
		return fmt.Errorf("failed to change to cloned directory %s: %v", tmpClonePath, err)
	}
	return nil
}

// cleanupTempClone removes the temporary clone directory
func cleanupTempClone(tmpClonePath string) error {
	if err := os.RemoveAll(tmpClonePath); err != nil {
		return fmt.Errorf("failed to clean up temporary clone directory %s: %v", tmpClonePath, err)
	}
	return nil
}

func getSHA1FromTaggedVersion(packageDir, versionTag string) (string, error) {
	getsha1Cmd := exec.Command("git", "rev-list", "-n", "1", versionTag)
	getsha1Cmd.Dir = packageDir
	sha1Output, err := getsha1Cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get SHA1 for tag '%s': %v", versionTag, err)
	}
	sha1 := strings.TrimSpace(string(sha1Output))
	return sha1, nil
}

// addPackageVersion adds a single version to the registry package directory
func addPackageVersion(packageDir, packageName, packageUUID, packageGitURL, sha1, versionTag string, project types.Project, registriesDir string) error {
	versionDir := filepath.Join(packageDir, versionTag)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("failed to create version directory %s: %v", versionDir, err)
	}

	specs := types.Specs{
		Name:    packageName,
		UUID:    packageUUID,
		Version: versionTag,
		GitURL:  packageGitURL,
		SHA1:    sha1,
		Deps:    project.Deps,
	}
	data, err := json.MarshalIndent(specs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal specs.json for version '%s': %v", versionTag, err)
	}
	specsFile := filepath.Join(versionDir, "specs.json")
	if err := os.WriteFile(specsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write specs.json for version '%s': %v", versionTag, err)
	}

	buildList, err := generateBuildList(project, registriesDir)
	if err != nil {
		return fmt.Errorf("failed to generate build list for version '%s': %v", versionTag, err)
	}
	data, err = json.MarshalIndent(buildList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal buildlist.json for version '%s': %v", versionTag, err)
	}
	buildListFile := filepath.Join(versionDir, "buildlist.json")
	if err := os.WriteFile(buildListFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write buildlist.json for version '%s': %v", versionTag, err)
	}
	return nil
}
