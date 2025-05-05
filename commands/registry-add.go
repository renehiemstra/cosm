package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// addPackageConfig holds configuration for adding a package to a registry
type addPackageConfig struct {
	registryName  string
	packageName   string
	versionTag    string
	packageGitURL string
	cosmDir       string
	registriesDir string
	registry      types.Registry
	registryFile  string
	packageUUID   string
	packageDir    string
	clonePath     string
	tags          []string
}

// RegistryAdd adds a package with all versions or a specific version to a registry
func RegistryAdd(cmd *cobra.Command, args []string) error {
	// Parse arguments and setup
	config, err := parseRegistryAddArgs(args)
	if err != nil {
		return err
	}

	// Update registry
	if err := updateSingleRegistry(config.registriesDir, config.registryName); err != nil {
		return err
	}

	// Load registry metadata
	config.registry, config.registryFile, err = LoadRegistryMetadata(config.registriesDir, config.registryName)
	if err != nil {
		return err
	}

	if config.versionTag == "" {
		// Mode 1: Add package with all versions
		return addPackageWithAllVersions(config)
	}
	// Mode 2: Add specific version
	return addSpecificPackageVersion(config)
}

// parseAddArgs validates arguments and sets up directories
func parseRegistryAddArgs(args []string) (*addPackageConfig, error) {
	if len(args) != 2 && len(args) != 3 {
		return nil, fmt.Errorf("requires two arguments (registry name, package giturl) or three arguments (registry name, package name, version)")
	}
	registryName := args[0]
	if registryName == "" {
		return nil, fmt.Errorf("registry name must not be empty")
	}
	cosmDir, err := getCosmDir()
	if err != nil {
		return nil, err
	}
	registriesDir := filepath.Join(cosmDir, "registries")
	if len(args) == 2 {
		packageGitURL := args[1]
		if packageGitURL == "" {
			return nil, fmt.Errorf("package giturl must not be empty")
		}
		return &addPackageConfig{
			registryName:  registryName,
			packageGitURL: packageGitURL,
			cosmDir:       cosmDir,
			registriesDir: registriesDir,
		}, nil
	}
	packageName := args[1]
	versionTag := args[2]
	if packageName == "" {
		return nil, fmt.Errorf("package name must not be empty")
	}
	if versionTag == "" || !strings.HasPrefix(versionTag, "v") {
		return nil, fmt.Errorf("version must be non-empty and start with 'v'")
	}
	return &addPackageConfig{
		registryName:  registryName,
		packageName:   packageName,
		versionTag:    versionTag,
		cosmDir:       cosmDir,
		registriesDir: registriesDir,
	}, nil
}

// addPackageWithAllVersions adds a package with all available versions to the registry
func addPackageWithAllVersions(config *addPackageConfig) error {
	// Clone package to temporary directory
	clonePath, err := clonePackageToTempDir(config.cosmDir, config.packageGitURL)
	if err != nil {
		return err
	}
	config.clonePath = clonePath
	defer cleanupTempClone(config.clonePath)

	// Fetch tags to ensure latest tags are available
	if _, err := gitCommand(config.clonePath, "fetch", "--tags"); err != nil {
		return fmt.Errorf("failed to fetch tags for repository at '%s': %v", config.packageGitURL, err)
	}

	// Validate Project.json to get package name and UUID
	project, err := loadProjectFromDir(config.clonePath)
	if err != nil {
		return err
	}
	err = validateProject(project)
	if err != nil {
		return err
	}
	config.packageName = project.Name
	config.packageUUID = project.UUID
	if err := ensurePackageNotRegistered(config.registry, config.packageName, config.registryName, config.clonePath); err != nil {
		return err
	}
	config.tags, err = validateAndCollectVersionTags(config.clonePath)
	if err != nil {
		return err
	}
	config.packageDir, err = setupPackageDir(config.registriesDir, config.registryName, config.packageName)
	if err != nil {
		return err
	}
	if len(config.tags) > 0 {
		// Update versions for all tags
		if err := updatePackageVersions(config.packageDir, config.packageName, config.packageUUID, config.packageGitURL, config.tags, config.registriesDir, config.clonePath); err != nil {
			return err
		}
	}

	// Update registry.json and move clone
	config.registry.Packages[config.packageName] = types.PackageInfo{
		UUID:   config.packageUUID,
		GitURL: config.packageGitURL,
	}
	if err := saveRegistryMetadata(config.registry, config.registryFile); err != nil {
		return err
	}
	config.clonePath, err = moveCloneToPermanentDir(config.cosmDir, config.clonePath, config.packageUUID)
	if err != nil {
		return err
	}
	commitMsg := fmt.Sprintf("Added package %s", config.packageName)
	if len(config.tags) > 0 {
		commitMsg = fmt.Sprintf("Added package %s version %s", config.packageName, config.tags[0])
	}
	if err := commitAndPushRegistryChanges(config.registriesDir, config.registryName, commitMsg); err != nil {
		return err
	}
	fmt.Printf("Added package '%s' to registry '%s'\n", config.packageName, config.registryName)
	return nil
}

// addSpecificPackageVersion adds a specific version of an existing package to the registry
func addSpecificPackageVersion(config *addPackageConfig) error {
	// Check if package exists in registry
	pkgInfo, exists := config.registry.Packages[config.packageName]
	if !exists {
		return fmt.Errorf("package '%s' not found in registry '%s'", config.packageName, config.registryName)
	}
	config.packageUUID = pkgInfo.UUID
	config.packageGitURL = pkgInfo.GitURL

	// Check if version is already registered
	config.packageDir = filepath.Join(config.registriesDir, config.registryName, strings.ToUpper(string(config.packageName[0])), config.packageName)
	versionsFile := filepath.Join(config.packageDir, "versions.json")
	var existingVersions []string
	if data, err := os.ReadFile(versionsFile); err == nil {
		if err := json.Unmarshal(data, &existingVersions); err != nil {
			return fmt.Errorf("failed to parse versions.json for package '%s': %v", config.packageName, err)
		}
		if contains(existingVersions, config.versionTag) {
			return fmt.Errorf("version '%s' of package '%s' is already registered in registry '%s'", config.versionTag, config.packageName, config.registryName)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read versions.json for package '%s': %v", config.packageName, err)
	}

	// Check if package is cloned
	config.clonePath = filepath.Join(config.cosmDir, "clones", config.packageUUID)
	if _, err := os.Stat(config.clonePath); os.IsNotExist(err) {
		tmpClonePath, err := clonePackageToTempDir(config.cosmDir, config.packageGitURL)
		if err != nil {
			return err
		}
		defer cleanupTempClone(tmpClonePath)
		config.clonePath, err = moveCloneToPermanentDir(config.cosmDir, tmpClonePath, config.packageUUID)
		if err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("failed to check clone at %s: %v", config.clonePath, err)
	}

	// Update versions for the specific tag
	if err := updatePackageVersions(config.packageDir, config.packageName, config.packageUUID, config.packageGitURL, []string{config.versionTag}, config.registriesDir, config.clonePath); err != nil {
		return err
	}

	// Commit and push registry changes
	commitMsg := fmt.Sprintf("Added version %s of package %s", config.versionTag, config.packageName)
	if err := commitAndPushRegistryChanges(config.registriesDir, config.registryName, commitMsg); err != nil {
		return err
	}

	fmt.Printf("Added version '%s' of package '%s' to registry '%s'\n", config.versionTag, config.packageName, config.registryName)
	return nil
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
func validateAndCollectVersionTags(clonePath string) ([]string, error) {
	tagOutput, err := gitCommand(clonePath, "tag")
	if err != nil || len(strings.TrimSpace(tagOutput)) == 0 {
		return []string{}, nil // No tags, return empty slice
	}

	tags := strings.Split(strings.TrimSpace(tagOutput), "\n")
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
			if err := fetchOrigin(clonePath); err != nil {
				return fmt.Errorf("failed to fetch remote changes for package '%s': %v", packageName, err)
			}

			// Checkout the specific version tag
			if err := checkoutVersion(clonePath, tag); err != nil {
				return fmt.Errorf("failed to checkout tag '%s' for package '%s': %v", tag, packageName, err)
			}

			// Load Project.json for this tag
			project, err := loadProjectFromDir(clonePath)
			if err != nil {
				return fmt.Errorf("failed to load Project.json for tag '%s': %v", tag, err)
			}

			// Validate project file
			if err := validateProject(project); err != nil {
				return fmt.Errorf("invalid Project.json for tag '%s': %v", tag, err)
			}

			// Revert clone to previous state
			if err := revertClone(clonePath); err != nil {
				return fmt.Errorf("failed to revert clone for tag '%s': %v", tag, err)
			}

			// Get SHA1 for the tag
			sha1Output, err := gitCommand(clonePath, "rev-list", "-n", "1", tag)
			if err != nil {
				return fmt.Errorf("failed to get SHA1 for tag '%s': %v", tag, err)
			}
			sha1 := strings.TrimSpace(sha1Output)

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

// addPackageVersion adds a single version to the registry package directory
func addPackageVersion(packageDir, packageName, packageUUID, packageGitURL, sha1, versionTag string, project *types.Project, registriesDir string) error {
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

// cleanupTempClone removes the temporary clone directory
func cleanupTempClone(tmpClonePath string) error {
	if tmpClonePath != "" {
		if err := os.RemoveAll(tmpClonePath); err != nil {
			return fmt.Errorf("failed to clean up temporary clone directory %s: %v", tmpClonePath, err)
		}
	}
	return nil
}
