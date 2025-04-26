package commands

import (
	"bufio"
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func Registry(cmd *cobra.Command, args []string) {
	fmt.Println("Registry command requires a subcommand (e.g., 'status', 'init').")
}

// RegistryStatus prints an overview of packages in a registry
func RegistryStatus(cmd *cobra.Command, args []string) error {
	registryName, err := validateStatusArgs(args) // Updated to handle error
	if err != nil {
		return err
	}
	cosmDir, err := getCosmDir() // Already returns error
	if err != nil {
		return err
	}
	registriesDir := setupRegistriesDir(cosmDir)
	if err := assertRegistryExists(registriesDir, registryName); err != nil { // Updated to handle error
		return err
	}
	registry, _, err := LoadRegistryMetadata(registriesDir, registryName) // Already returns error
	if err != nil {
		return err
	}
	printRegistryStatus(registryName, registry) // No error return needed, prints to stdout
	return nil
}

// RegistryInit initializes a new package registry
func RegistryInit(cmd *cobra.Command, args []string) error {
	originalDir, registryName, gitURL, registriesDir, err := setupAndParseInitArgs(args) // Updated to handle error
	if err != nil {
		return err
	}
	registryNames, err := loadAndCheckRegistries(registriesDir, registryName) // Updated to handle error
	if err != nil {
		return err
	}
	registrySubDir, err := cloneAndEnterRegistry(registriesDir, registryName, gitURL) // Updated to handle error
	if err != nil {
		return err
	}
	if err := ensureDirectoryEmpty(registrySubDir, gitURL); err != nil { // Updated to handle error
		cleanupInit(originalDir, registrySubDir, true)
		return err
	}
	if err := updateRegistriesList(registriesDir, registryNames, registryName); err != nil { // Updated to handle error
		cleanupInit(originalDir, registrySubDir, true)
		return err
	}
	_, err = initializeRegistryMetadata(registrySubDir, registryName, gitURL) // Updated to handle error
	if err != nil {
		cleanupInit(originalDir, registrySubDir, true)
		return err
	}
	if err := commitAndPushInitialRegistryChanges(registryName, gitURL); err != nil { // Updated to handle error
		cleanupInit(originalDir, registrySubDir, true)
		return err
	}
	if err := restoreOriginalDir(originalDir); err != nil { // Updated to handle error
		return err
	}
	fmt.Printf("Initialized registry '%s' with Git URL: %s\n", registryName, gitURL)
	return nil
}

// RegistryAdd adds a package version to a registry
func RegistryAdd(cmd *cobra.Command, args []string) error {
	registryName, packageGitURL, cosmDir, registriesDir, err := parseArgsAndSetup(args)
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
	tmpClonePath, err := clonePackageToTempDir(cosmDir, packageGitURL)
	if err != nil {
		return err
	}
	// Capture currentDir before entering clone
	currentDir, err := os.Getwd()
	if err != nil {
		cleanupRegistryAdd(currentDir, tmpClonePath)
		return fmt.Errorf("failed to get current directory: %v", err)
	}
	if err := enterCloneDir(tmpClonePath); err != nil {
		cleanupRegistryAdd(currentDir, tmpClonePath)
		return err
	}
	project, err := validateProjectFile(packageGitURL)
	if err != nil {
		cleanupRegistryAdd(currentDir, tmpClonePath)
		return err
	}
	if err := ensurePackageNotRegistered(registry, project.Name, registryName, tmpClonePath); err != nil {
		cleanupRegistryAdd(currentDir, tmpClonePath)
		return err
	}
	validTags, err := validateAndCollectVersionTags(packageGitURL, project.Version)
	if err != nil {
		cleanupRegistryAdd(currentDir, tmpClonePath)
		return err
	}
	packageDir, err := setupPackageDir(registriesDir, registryName, project.Name)
	if err != nil {
		cleanupRegistryAdd(currentDir, tmpClonePath)
		return err
	}
	if err := updatePackageVersions(packageDir, project.Name, project.UUID, packageGitURL, validTags, project, registriesDir); err != nil {
		cleanupRegistryAdd(currentDir, tmpClonePath)
		return err
	}
	if err := finalizePackageAddition(cosmDir, tmpClonePath, project.UUID, registriesDir, registryName, project.Name, registry, registryMetaFile, validTags[0]); err != nil {
		cleanupRegistryAdd(currentDir, tmpClonePath)
		return err
	}
	if err := restoreRegistryAddDir(currentDir); err != nil {
		cleanupRegistryAdd(currentDir, tmpClonePath)
		return err
	}
	cleanupRegistryAdd(currentDir, tmpClonePath) // Ensure tmpClonePath is removed in success path
	fmt.Printf("Added package '%s' with version '%s' to registry '%s'\n", project.Name, project.Version, registryName)
	return nil
}

// RegistryRm removes a package or a specific version from a registry
func RegistryRm(cmd *cobra.Command, args []string) error {
	registryName, packageName, version, err := validateRmArgs(args)
	if err != nil {
		return err
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return err
	}
	registriesDir := setupRegistriesDir(cosmDir)

	// Load registry metadata
	registry, registryMetaFile, err := LoadRegistryMetadata(registriesDir, registryName)
	if err != nil {
		return err
	}

	// Check if package exists
	if _, exists := registry.Packages[packageName]; !exists {
		return fmt.Errorf("package '%s' not found in registry '%s'", packageName, registryName)
	}

	// Confirm removal
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		if err := confirmRemoval(registryName, packageName, version); err != nil {
			return err
		}
	}

	// Remove package or version
	packageDir := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName)
	if err := removePackageVersion(registriesDir, registryName, packageName, packageDir, version, &registry); err != nil {
		return err
	}

	// Update registry.json
	if err := updateRegistryMetadata(registry, registryMetaFile, registryName); err != nil {
		return err
	}

	// Commit and push changes to remote
	if err := commitAndPushRemoval(registriesDir, registryName, packageName, version); err != nil {
		return err
	}

	if version != "" {
		fmt.Printf("Removed version '%s' of package '%s' from registry '%s'\n", version, packageName, registryName)
	} else {
		fmt.Printf("Removed package '%s' from registry '%s'\n", packageName, registryName)
	}
	return nil
}

// RegistryDelete deletes a registry from the local system
func RegistryDelete(cmd *cobra.Command, args []string) error {
	registryName, err := validateStatusArgs(args) // Reusing validateStatusArgs for single argument check
	if err != nil {
		return err
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return err
	}
	registriesDir := setupRegistriesDir(cosmDir)

	// Check if registry exists
	if err := assertRegistryExists(registriesDir, registryName); err != nil {
		return err
	}

	registryPath := filepath.Join(registriesDir, registryName)
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		return fmt.Errorf("registry directory '%s' not found", registryName)
	}

	// Check for --force flag
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Printf("Are you sure you want to delete registry '%s'? [y/N]: ", registryName)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if response != "y" && response != "yes" {
			fmt.Println("Registry deletion cancelled.")
			return nil
		}
	}

	// Remove registry directory
	if err := os.RemoveAll(registryPath); err != nil {
		return fmt.Errorf("failed to delete registry directory '%s': %v", registryPath, err)
	}

	// Update registries.json
	registryNames, err := loadRegistryNames(cosmDir)
	if err != nil {
		return err
	}
	var updatedNames []string
	for _, name := range registryNames {
		if name != registryName {
			updatedNames = append(updatedNames, name)
		}
	}
	data, err := json.MarshalIndent(updatedNames, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registries.json: %v", err)
	}
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if err := os.WriteFile(registriesFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registries.json: %v", err)
	}

	fmt.Printf("Deleted registry '%s'\n", registryName)
	return nil
}

// RegistryClone clones a registry from a Git URL
func RegistryClone(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one argument required (e.g., cosm registry clone <giturl>)")
	}
	gitURL := args[0]
	if gitURL == "" {
		return fmt.Errorf("git URL cannot be empty")
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return err
	}
	registriesDir := setupRegistriesDir(cosmDir)

	// Clone to a temporary directory to read registry.json
	tmpDir, err := os.MkdirTemp("", "cosm-clone-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := exec.Command("git", "clone", gitURL, tmpDir).Run(); err != nil {
		return fmt.Errorf("failed to clone repository at '%s': %v", gitURL, err)
	}

	// Read registry.json to get registry name
	registryMetaFile := filepath.Join(tmpDir, "registry.json")
	data, err := os.ReadFile(registryMetaFile)
	if err != nil {
		return fmt.Errorf("failed to read registry.json from cloned repository: %v", err)
	}
	var registry types.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return fmt.Errorf("failed to parse registry.json: %v", err)
	}
	if registry.Name == "" {
		return fmt.Errorf("registry.json does not contain a valid registry name")
	}

	// Check for existing registry
	registryNames, err := loadRegistryNames(cosmDir)
	if err != nil {
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no registries available") {
			return err
		}
		registryNames = []string{} // Initialize empty list if registries.json is missing or empty
	}
	registryPath := filepath.Join(registriesDir, registry.Name)
	if contains(registryNames, registry.Name) {
		fmt.Printf("Registry '%s' already exists. Overwrite? [y/N]: ", registry.Name)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if response != "y" && response != "yes" {
			fmt.Println("Registry clone cancelled.")
			return nil
		}
		// Delete existing registry (without --force to align with prompt)
		deleteCmd := &cobra.Command{}
		if err := RegistryDelete(deleteCmd, []string{registry.Name}); err != nil {
			return fmt.Errorf("failed to delete existing registry '%s': %v", registry.Name, err)
		}
	}

	// Clone to final location
	if err := exec.Command("git", "clone", gitURL, registryPath).Run(); err != nil {
		return fmt.Errorf("failed to clone repository to '%s': %v", registryPath, err)
	}

	// Update registries.json
	registryNames = append(registryNames, registry.Name)
	data, err = json.MarshalIndent(registryNames, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registries.json: %v", err)
	}
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if err := os.WriteFile(registriesFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registries.json: %v", err)
	}

	fmt.Printf("Cloned registry '%s' from %s\n", registry.Name, gitURL)
	return nil
}

// RegistryUpdate updates and synchronizes a registry or all registries with their remotes
func RegistryUpdate(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	if all && len(args) != 0 {
		return fmt.Errorf("no arguments allowed with --all flag")
	}
	if !all && len(args) != 1 {
		return fmt.Errorf("exactly one argument required (e.g., cosm registry update <registry_name>)")
	}

	cosmDir, err := getCosmDir()
	if err != nil {
		return err
	}
	registriesDir := setupRegistriesDir(cosmDir)

	if all {
		registryNames, err := loadRegistryNames(cosmDir)
		if err != nil {
			return fmt.Errorf("failed to load registry names: %v", err)
		}
		if len(registryNames) == 0 {
			fmt.Println("No registries to update.")
			return nil
		}
		for _, name := range registryNames {
			if err := updateSingleRegistry(registriesDir, name); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to update registry '%s': %v\n", name, err)
				continue
			}
			fmt.Printf("Updated registry '%s'\n", name)
		}
		return nil
	}

	registryName := args[0]
	if err := updateSingleRegistry(registriesDir, registryName); err != nil {
		return err
	}
	fmt.Printf("Updated registry '%s'\n", registryName)
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

// cleanupTempClone removes the temporary clone directory
func cleanupTempClone(tmpClonePath string) error {
	if err := os.RemoveAll(tmpClonePath); err != nil {
		return fmt.Errorf("failed to clean up temporary clone directory %s: %v", tmpClonePath, err)
	}
	return nil
}

// clonePackageToTempDir creates a temp clone directly in the clones directory
func clonePackageToTempDir(cosmDir, packageGitURL string) (string, error) {
	clonesDir := filepath.Join(cosmDir, "clones")
	if err := os.MkdirAll(clonesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create clones directory: %v", err)
	}
	tmpClonePath := filepath.Join(clonesDir, "tmp-clone-"+uuid.New().String())

	if err := exec.Command("git", "clone", packageGitURL, tmpClonePath).Run(); err != nil {
		cloneOutput, _ := exec.Command("git", "clone", packageGitURL, tmpClonePath).CombinedOutput()
		cleanupErr := cleanupTempClone(tmpClonePath)
		if cleanupErr != nil {
			return "", fmt.Errorf("failed to clone package repository at '%s': %v; cleanup failed: %v\nOutput: %s", packageGitURL, err, cleanupErr, cloneOutput)
		}
		return "", fmt.Errorf("failed to clone package repository at '%s': %v\nOutput: %s", packageGitURL, err, cloneOutput)
	}
	return tmpClonePath, nil
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

// assertRegistryExists verifies that the specified registry exists in registries.json
func assertRegistryExists(registriesDir, registryName string) error {
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if _, err := os.Stat(registriesFile); os.IsNotExist(err) {
		return fmt.Errorf("no registries found (run 'cosm registry init' first)")
	}
	var registryNames []string
	data, err := os.ReadFile(registriesFile)
	if err != nil {
		return fmt.Errorf("failed to read registries.json: %v", err)
	}
	if err := json.Unmarshal(data, &registryNames); err != nil {
		return fmt.Errorf("failed to parse registries.json: %v", err)
	}
	for _, name := range registryNames {
		if name == registryName {
			return nil
		}
	}
	return fmt.Errorf("registry '%s' not found in registries.json", registryName)
}

// cleanupPull reverts to the original directory
func cleanupPull(originalDir string) {
	if err := os.Chdir(originalDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to return to original directory %s: %v\n", originalDir, err)
	}
}

// restorePullDir returns to the original directory
func restorePullDir(originalDir string) error {
	if err := os.Chdir(originalDir); err != nil {
		return fmt.Errorf("failed to return to original directory %s: %v", originalDir, err)
	}
	return nil
}

// LoadRegistryMetadata loads and validates the registry metadata from registry.json
func LoadRegistryMetadata(registriesDir, registryName string) (types.Registry, string, error) {
	registryMetaFile := filepath.Join(registriesDir, registryName, "registry.json")
	data, err := os.ReadFile(registryMetaFile)
	if err != nil {
		return types.Registry{}, "", fmt.Errorf("failed to read registry.json for '%s': %v", registryName, err)
	}
	var registry types.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return types.Registry{}, "", fmt.Errorf("failed to parse registry.json for '%s': %v", registryName, err)
	}
	if registry.Packages == nil {
		registry.Packages = make(map[string]string)
	}
	return registry, registryMetaFile, nil
}

// commitAndPushRegistryChanges commits and pushes changes to the registry's Git repository
func commitAndPushRegistryChanges(registriesDir, registryName, packageName, versionTag string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}
	registryDir := filepath.Join(registriesDir, registryName)
	if err := os.Chdir(registryDir); err != nil {
		cleanupCommit(currentDir)
		return fmt.Errorf("failed to change to registry directory %s: %v", registryDir, err)
	}
	addCmd := exec.Command("git", "add", ".")
	addOutput, err := addCmd.CombinedOutput()
	if err != nil {
		cleanupCommit(currentDir)
		return fmt.Errorf("failed to stage changes in registry: %v\nOutput: %s", err, addOutput)
	}
	commitMsg := fmt.Sprintf("Added package %s version %s", packageName, versionTag)
	commitCmd := exec.Command("git", "commit", "-m", commitMsg)
	commitOutput, err := commitCmd.CombinedOutput()
	if err != nil {
		cleanupCommit(currentDir)
		return fmt.Errorf("failed to commit changes in registry: %v\nOutput: %s", err, commitOutput)
	}
	pushCmd := exec.Command("git", "push", "origin", "main")
	pushOutput, err := pushCmd.CombinedOutput()
	if err != nil {
		cleanupCommit(currentDir)
		return fmt.Errorf("failed to push changes to registry: %v\nOutput: %s", err, pushOutput)
	}
	if err := restoreCommitDir(currentDir); err != nil {
		return err
	}
	return nil
}

// cleanupCommit reverts to the original directory
func cleanupCommit(originalDir string) {
	if err := os.Chdir(originalDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to return to original directory %s: %v\n", originalDir, err)
	}
}

// restoreCommitDir returns to the original directory
func restoreCommitDir(originalDir string) error {
	if err := os.Chdir(originalDir); err != nil {
		return fmt.Errorf("failed to return to original directory %s: %v", originalDir, err)
	}
	return nil
}

// validateProjectFile reads and validates Project.json, returning the project
func validateProjectFile(packageGitURL string) (types.Project, error) {
	data, err := os.ReadFile("Project.json")
	if err != nil {
		return types.Project{}, fmt.Errorf("repository at '%s' does not contain a Project.json file: %v", packageGitURL, err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return types.Project{}, fmt.Errorf("invalid Project.json in repository at '%s': %v", packageGitURL, err)
	}
	if project.Name == "" {
		return types.Project{}, fmt.Errorf("Project.json in repository at '%s' does not contain a valid package name", packageGitURL)
	}
	if project.UUID == "" {
		return types.Project{}, fmt.Errorf("Project.json in repository at '%s' does not contain a valid UUID", packageGitURL)
	}
	if _, err := uuid.Parse(project.UUID); err != nil {
		return types.Project{}, fmt.Errorf("invalid UUID '%s' in Project.json at '%s': %v", project.UUID, packageGitURL, err)
	}
	if project.Version == "" {
		return types.Project{}, fmt.Errorf("Project.json at '%s' does not contain a version", packageGitURL)
	}
	// Validate version parsing
	_, err = ParseSemVer(project.Version) // Fixed to handle both return values
	if err != nil {
		return types.Project{}, fmt.Errorf("invalid version in Project.json at '%s': %v", packageGitURL, err)
	}
	return project, nil
}

// validateAndCollectVersionTags fetches Git tags, or releases the current version if none exist
func validateAndCollectVersionTags(packageGitURL string, packageVersion string) ([]string, error) {
	tagOutput, err := exec.Command("git", "tag").CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(tagOutput))) == 0 {
		// No tags found, use Project.json packageVersion and tag it
		if packageVersion == "" {
			return nil, fmt.Errorf("project.json at '%s' has no version specified", packageGitURL)
		}

		// Tag the current version
		if err := exec.Command("git", "tag", packageVersion).Run(); err != nil {
			return nil, fmt.Errorf("failed to tag version '%s' in repository at '%s': %v", packageVersion, packageGitURL, err)
		}
		// Push the tag to the remote
		if err := exec.Command("git", "push", "origin", packageVersion).Run(); err != nil {
			return nil, fmt.Errorf("failed to push tag '%s' to origin for repository at '%s': %v", packageVersion, packageGitURL, err)
		}
		fmt.Fprintf(os.Stderr, "No valid tags found; released version '%s' from Project.json to repository at '%s'\n", packageVersion, packageGitURL)
		return []string{packageVersion}, nil
	}

	tags := strings.Split(strings.TrimSpace(string(tagOutput)), "\n")
	var validTags []string
	for _, tag := range tags {
		if strings.HasPrefix(tag, "v") && len(strings.Split(tag, ".")) >= 2 {
			validTags = append(validTags, tag)
		}
	}
	if len(validTags) == 0 {
		return nil, fmt.Errorf("no valid version tags (e.g., vX.Y.Z) found in repository at '%s'", packageGitURL)
	}
	return validTags, nil
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// addPackageVersion adds a single version to the package directory
func addPackageVersion(packageDir, packageName, packageUUID, packageGitURL, versionTag string, project types.Project, registriesDir string) error {
	versionDir := filepath.Join(packageDir, versionTag)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("failed to create version directory %s: %v", versionDir, err)
	}

	sha1Output, err := exec.Command("git", "rev-list", "-n", "1", versionTag).Output()
	if err != nil {
		return fmt.Errorf("failed to get SHA1 for tag '%s': %v", versionTag, err)
	}
	sha1 := strings.TrimSpace(string(sha1Output))

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

// generateBuildList creates a build list using Minimum Version Selection (MVS),
// including direct dependencies from project.Deps and transitive dependencies
// from dependency build lists, taking the maximum version for shared dependencies.
func generateBuildList(project types.Project, registriesDir string) (types.BuildList, error) {
	buildList := types.BuildList{Dependencies: make(map[string]types.BuildListDependency)}

	// Process direct dependencies
	for depName, depVersion := range project.Deps {
		depUUID, specs, depBuildList, err := findDependency(depName, depVersion, registriesDir)
		if err != nil {
			return types.BuildList{}, err
		}
		key, entry, err := createDependencyEntry(depName, depVersion, depUUID, specs)
		if err != nil {
			return types.BuildList{}, err
		}
		if err := mergeDependencyEntry(&buildList, key, entry); err != nil {
			return types.BuildList{}, err
		}
		// Process transitive dependencies
		for transKey, transDep := range depBuildList.Dependencies {
			if err := mergeDependencyEntry(&buildList, transKey, transDep); err != nil {
				return types.BuildList{}, err
			}
		}
	}

	return buildList, nil
}

// findDependency searches all registries for a dependency, returning its UUID, specs, and build list
func findDependency(depName, depVersion, registriesDir string) (string, types.Specs, types.BuildList, error) {
	registriesFile := filepath.Join(registriesDir, "registries.json")
	var registryNames []string
	if data, err := os.ReadFile(registriesFile); err == nil {
		if err := json.Unmarshal(data, &registryNames); err != nil {
			return "", types.Specs{}, types.BuildList{}, fmt.Errorf("failed to parse registries.json: %v", err)
		}
	} else if !os.IsNotExist(err) {
		return "", types.Specs{}, types.BuildList{}, fmt.Errorf("failed to read registries.json: %v", err)
	}

	for _, regName := range registryNames {
		reg, _, err := LoadRegistryMetadata(registriesDir, regName)
		if err != nil {
			continue
		}
		if uuid, exists := reg.Packages[depName]; exists {
			specs, err := loadSpecs(registriesDir, regName, depName, depVersion)
			if err != nil {
				continue
			}
			if specs.Version != depVersion {
				continue
			}
			buildList, err := loadBuildList(registriesDir, regName, depName, depVersion)
			if err != nil {
				return "", types.Specs{}, types.BuildList{}, fmt.Errorf("failed to load build list for '%s@%s' in registry '%s': %v", depName, depVersion, regName, err)
			}
			return uuid, specs, buildList, nil
		}
	}
	return "", types.Specs{}, types.BuildList{}, fmt.Errorf("dependency '%s@%s' not found in any registry", depName, depVersion)
}

// createDependencyEntry builds a BuildListDependency entry with its key
func createDependencyEntry(depName, depVersion, depUUID string, specs types.Specs) (string, types.BuildListDependency, error) {
	majorVersion, err := GetMajorVersion(depVersion)
	if err != nil {
		return "", types.BuildListDependency{}, fmt.Errorf("failed to get major version for '%s@%s': %v", depName, depVersion, err)
	}
	key := fmt.Sprintf("%s@%s", depUUID, majorVersion)
	entry := types.BuildListDependency{
		Name:    depName,
		UUID:    depUUID,
		Version: depVersion,
		GitURL:  specs.GitURL,
		SHA1:    specs.SHA1,
	}
	return key, entry, nil
}

// mergeDependencyEntry adds or updates a dependency in the build list, keeping the higher version
func mergeDependencyEntry(buildList *types.BuildList, key string, entry types.BuildListDependency) error {
	if currEntry, exists := buildList.Dependencies[key]; exists {
		maxVersion, err := MaxSemVer(currEntry.Version, entry.Version)
		if err != nil {
			return fmt.Errorf("failed to compare versions for '%s': %v", entry.Name, err)
		}
		if maxVersion == entry.Version {
			buildList.Dependencies[key] = entry
		}
	} else {
		buildList.Dependencies[key] = entry
	}
	return nil
}

// loadBuildList loads a package's build list from buildlist.json
func loadBuildList(registriesDir, registryName, packageName, version string) (types.BuildList, error) {
	buildListFile := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName, version, "buildlist.json")
	data, err := os.ReadFile(buildListFile)
	if err != nil {
		if os.IsNotExist(err) {
			return types.BuildList{Dependencies: make(map[string]types.BuildListDependency)}, nil // No build list yet
		}
		return types.BuildList{}, fmt.Errorf("failed to read buildlist.json: %v", err)
	}
	var buildList types.BuildList
	if err := json.Unmarshal(data, &buildList); err != nil {
		return types.BuildList{}, fmt.Errorf("failed to parse buildlist.json: %v", err)
	}
	return buildList, nil
}

// loadSpecs loads a package's specs from specs.json
func loadSpecs(registriesDir, registryName, packageName, version string) (types.Specs, error) {
	specsFile := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName, version, "specs.json")
	data, err := os.ReadFile(specsFile)
	if err != nil {
		return types.Specs{}, fmt.Errorf("failed to read specs.json: %v", err)
	}
	var specs types.Specs
	if err := json.Unmarshal(data, &specs); err != nil {
		return types.Specs{}, fmt.Errorf("failed to parse specs.json: %v", err)
	}
	return specs, nil
}

// setupAndParseInitArgs validates arguments and sets up directories for RegistryInit
func setupAndParseInitArgs(args []string) (string, string, string, string, error) {
	originalDir, err := os.Getwd()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get original directory: %v", err)
	}

	if len(args) != 2 {
		return "", "", "", "", fmt.Errorf("exactly two arguments required (e.g., cosm registry init <registry name> <giturl>)")
	}
	registryName := args[0]
	gitURL := args[1]

	if registryName == "" {
		return "", "", "", "", fmt.Errorf("registry name cannot be empty")
	}
	if gitURL == "" {
		return "", "", "", "", fmt.Errorf("git URL cannot be empty")
	}

	cosmDir, err := getGlobalCosmDir()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get global .cosm directory: %v", err)
	}
	registriesDir := filepath.Join(cosmDir, "registries")
	if err := os.MkdirAll(registriesDir, 0755); err != nil {
		return "", "", "", "", fmt.Errorf("failed to create %s directory: %v", registriesDir, err)
	}

	return originalDir, registryName, gitURL, registriesDir, nil
}

// loadAndCheckRegistries loads registries.json and checks for duplicate registry names
func loadAndCheckRegistries(registriesDir, registryName string) ([]string, error) {
	registriesFile := filepath.Join(registriesDir, "registries.json")
	var registryNames []string
	if data, err := os.ReadFile(registriesFile); err == nil {
		if err := json.Unmarshal(data, &registryNames); err != nil {
			return nil, fmt.Errorf("failed to parse registries.json: %v", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read registries.json: %v", err)
	}

	for _, name := range registryNames {
		if name == registryName {
			return nil, fmt.Errorf("registry '%s' already exists", registryName)
		}
	}

	return registryNames, nil
}

// cleanupInit reverts to the original directory and removes the registrySubDir if needed
func cleanupInit(originalDir, registrySubDir string, removeDir bool) {
	if err := os.Chdir(originalDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error returning to original directory %s: %v\n", originalDir, err)
		// Don’t exit here; let the caller handle the exit after cleanup
	}
	if removeDir {
		if err := os.RemoveAll(registrySubDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to clean up registry directory %s: %v\n", registrySubDir, err)
		}
	}
}

// cloneAndEnterRegistry clones the repository into registries/<registryName> and changes to it
func cloneAndEnterRegistry(registriesDir, registryName, gitURL string) (string, error) {
	registrySubDir := filepath.Join(registriesDir, registryName)
	cloneCmd := exec.Command("git", "clone", gitURL, registrySubDir)
	cloneOutput, err := cloneCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to clone repository at '%s' into %s: %v\nOutput: %s", gitURL, registrySubDir, err, cloneOutput)
	}

	// Change to the cloned directory
	if err := os.Chdir(registrySubDir); err != nil {
		return "", fmt.Errorf("failed to change to registry directory %s: %v", registrySubDir, err)
	}
	return registrySubDir, nil
}

// ensureDirectoryEmpty checks if the cloned directory is empty except for .git
func ensureDirectoryEmpty(dir, gitURL string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %v", dir, err)
	}
	for _, file := range files {
		if file.Name() != ".git" { // Ignore .git directory
			return fmt.Errorf("repository at '%s' cloned into %s is not empty (contains %s)", gitURL, dir, file.Name())
		}
	}
	return nil
}

// updateRegistriesList adds the registry name to registries.json
func updateRegistriesList(registriesDir string, registryNames []string, registryName string) error {
	registryNames = append(registryNames, registryName)
	data, err := json.MarshalIndent(registryNames, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registries.json: %v", err)
	}
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if err := os.WriteFile(registriesFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registries.json: %v", err)
	}
	return nil
}

// initializeRegistryMetadata creates and writes the registry.json file
func initializeRegistryMetadata(registrySubDir, registryName, gitURL string) (string, error) {
	registryMetaFile := filepath.Join(registrySubDir, "registry.json")
	registry := types.Registry{
		Name:     registryName,
		UUID:     uuid.New().String(),
		GitURL:   gitURL,
		Packages: make(map[string]string),
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal registry.json: %v", err)
	}
	if err := os.WriteFile(registryMetaFile, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write registry.json: %v", err)
	}
	return registryMetaFile, nil
}

// commitAndPushInitialRegistryChanges stages, commits, and pushes the initial registry changes
func commitAndPushInitialRegistryChanges(registryName, gitURL string) error {
	addCmd := exec.Command("git", "add", "registry.json")
	addOutput, err := addCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stage registry.json: %v\nOutput: %s", err, addOutput)
	}
	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("Initialized registry %s", registryName))
	commitOutput, err := commitCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit initial registry setup: %v\nOutput: %s", err, commitOutput)
	}
	pushCmd := exec.Command("git", "push", "origin", "main")
	pushOutput, err := pushCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push initial commit to %s: %v\nOutput: %s", gitURL, err, pushOutput)
	}
	return nil
}

// restoreOriginalDir returns to the original directory without removing the registry subdir
func restoreOriginalDir(originalDir string) error {
	if err := os.Chdir(originalDir); err != nil {
		return fmt.Errorf("failed to return to original directory %s: %v", originalDir, err)
	}
	return nil
}

// parseArgsAndSetup validates arguments and sets up directories for RegistryAdd
func parseArgsAndSetup(args []string) (string, string, string, string, error) {
	if len(args) != 2 {
		return "", "", "", "", fmt.Errorf("exactly two arguments required (e.g., cosm registry add <registry_name> <package giturl>)")
	}
	registryName := args[0]
	packageGitURL := args[1]

	if registryName == "" {
		return "", "", "", "", fmt.Errorf("registry name cannot be empty")
	}
	if packageGitURL == "" {
		return "", "", "", "", fmt.Errorf("package Git URL cannot be empty")
	}

	cosmDir, err := getGlobalCosmDir()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get global .cosm directory: %v", err)
	}
	registriesDir := filepath.Join(cosmDir, "registries")

	return registryName, packageGitURL, cosmDir, registriesDir, nil
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
func updatePackageVersions(packageDir, packageName, packageUUID, packageGitURL string, tags []string, project types.Project, registriesDir string) error {
	versionsFile := filepath.Join(packageDir, "versions.json")
	var versions []string
	if data, err := os.ReadFile(versionsFile); err == nil {
		if err := json.Unmarshal(data, &versions); err != nil {
			return fmt.Errorf("failed to parse versions.json for package '%s': %v", packageName, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read versions.json for package '%s': %v", packageName, err)
	}
	for _, tag := range tags {
		if !contains(versions, tag) {
			versions = append(versions, tag)
			if err := addPackageVersion(packageDir, packageName, packageUUID, packageGitURL, tag, project, registriesDir); err != nil {
				return err
			}
		}
	}
	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal versions.json for package '%s': %v", packageName, err)
	}
	if err := os.WriteFile(versionsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write versions.json for package '%s': %v", packageName, err)
	}
	return nil
}

// finalizePackageAddition updates the registry.json and moves the clone to a permanent location
func finalizePackageAddition(cosmDir, tmpClonePath, packageUUID string, registriesDir, registryName, packageName string, registry types.Registry, registryMetaFile string, versionTag string) error {
	registry.Packages[packageName] = packageUUID
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry.json for '%s': %v", registryName, err)
	}
	if err := os.WriteFile(registryMetaFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry.json for '%s': %v", registryName, err)
	}
	_, err = moveCloneToPermanentDir(cosmDir, tmpClonePath, packageUUID)
	if err != nil {
		return err
	}
	// Commit and push changes to the registry
	if err := os.Chdir(filepath.Join(registriesDir, registryName)); err != nil {
		return fmt.Errorf("failed to change to registry directory %s: %v", registryName, err)
	}
	addCmd := exec.Command("git", "add", ".")
	if addOutput, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage registry changes: %v\nOutput: %s", err, addOutput)
	}
	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("Added package %s version %s", packageName, versionTag))
	if commitOutput, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to commit registry changes: %v\nOutput: %s", err, commitOutput)
	}
	pushCmd := exec.Command("git", "push", "origin", "main")
	if pushOutput, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push registry changes: %v\nOutput: %s", err, pushOutput)
	}
	return nil
}

// validateStatusArgs checks the command-line arguments for validity
func validateStatusArgs(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("exactly one argument required (e.g., cosm registry status <registry_name>)")
	}
	registryName := args[0]
	if registryName == "" {
		return "", fmt.Errorf("registry name cannot be empty")
	}
	return registryName, nil
}

// setupRegistriesDir constructs the registries directory path
func setupRegistriesDir(cosmDir string) string {
	return filepath.Join(cosmDir, "registries")
}

// printRegistryStatus displays the registry's package information
func printRegistryStatus(registryName string, registry types.Registry) {
	fmt.Printf("Registry Status for '%s':\n", registryName)
	if len(registry.Packages) == 0 {
		fmt.Println("  No packages registered.")
	} else {
		fmt.Println("  Packages:")
		for pkgName, pkgUUID := range registry.Packages {
			fmt.Printf("    - %s (UUID: %s)\n", pkgName, pkgUUID)
		}
	}
}

// updateSingleRegistry pulls updates for a single registry
func updateSingleRegistry(registriesDir, registryName string) error {
	if err := assertRegistryExists(registriesDir, registryName); err != nil {
		return err
	}
	registryDir := filepath.Join(registriesDir, registryName)
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(registryDir); err != nil {
		cleanupPull(currentDir)
		return fmt.Errorf("failed to change to registry directory %s: %v", registryDir, err)
	}
	pullCmd := exec.Command("git", "pull", "origin", "main")
	pullOutput, err := pullCmd.CombinedOutput()
	if err != nil {
		cleanupPull(currentDir)
		return fmt.Errorf("failed to pull updates for registry '%s': %v\nOutput: %s", registryName, err, pullOutput)
	}
	if err := restorePullDir(currentDir); err != nil {
		return err
	}
	return nil
}

// validateRmArgs validates the arguments for RegistryRm
func validateRmArgs(args []string) (registryName, packageName, version string, err error) {
	if len(args) < 2 || len(args) > 3 {
		return "", "", "", fmt.Errorf("expected 2 or 3 arguments (e.g., cosm registry rm <registry_name> <package_name> [v<version>] [--force])")
	}
	registryName = args[0]
	packageName = args[1]
	if len(args) == 3 {
		version = args[2]
		if !strings.HasPrefix(version, "v") {
			return "", "", "", fmt.Errorf("version '%s' must start with 'v'", version)
		}
	}
	return registryName, packageName, version, nil
}

// confirmRemoval prompts the user for confirmation if --force is not set
func confirmRemoval(registryName, packageName, version string) error {
	prompt := fmt.Sprintf("Are you sure you want to remove package '%s' from registry '%s'?", packageName, registryName)
	if version != "" {
		prompt = fmt.Sprintf("Are you sure you want to remove version '%s' of package '%s' from registry '%s'?", version, packageName, registryName)
	}
	fmt.Printf("%s [y/N]: ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	response := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if response != "y" && response != "yes" {
		fmt.Println("Package removal cancelled.")
		return fmt.Errorf("operation cancelled by user")
	}
	return nil
}

// removePackageVersion removes a specific version or entire package from the registry
func removePackageVersion(registriesDir, registryName, packageName, packageDir, version string, registry *types.Registry) error {
	if version != "" {
		versionsFile := filepath.Join(packageDir, "versions.json")
		var versions []string
		if data, err := os.ReadFile(versionsFile); err == nil {
			if err := json.Unmarshal(data, &versions); err != nil {
				return fmt.Errorf("failed to parse versions.json for package '%s': %v", packageName, err)
			}
		} else {
			return fmt.Errorf("versions.json not found for package '%s' in registry '%s'", packageName, registryName)
		}

		// Check if version exists
		var updatedVersions []string
		found := false
		for _, v := range versions {
			if v == version {
				found = true
				continue
			}
			updatedVersions = append(updatedVersions, v)
		}
		if !found {
			return fmt.Errorf("version '%s' not found for package '%s' in registry '%s'", version, packageName, registryName)
		}

		// Remove version directory
		versionDir := filepath.Join(packageDir, version)
		if err := os.RemoveAll(versionDir); err != nil {
			return fmt.Errorf("failed to remove version directory '%s': %v", versionDir, err)
		}

		// Update versions.json
		data, err := json.MarshalIndent(updatedVersions, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal versions.json for package '%s': %v", packageName, err)
		}
		if err := os.WriteFile(versionsFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write versions.json for package '%s': %v", packageName, err)
		}

		// If no versions remain, remove the entire package
		if len(updatedVersions) == 0 {
			delete(registry.Packages, packageName)
			if err := os.RemoveAll(packageDir); err != nil {
				return fmt.Errorf("failed to remove package directory '%s': %v", packageDir, err)
			}
		}
	} else {
		// Remove entire package
		delete(registry.Packages, packageName)
		if err := os.RemoveAll(packageDir); err != nil {
			return fmt.Errorf("failed to remove package directory '%s': %v", packageDir, err)
		}
	}
	return nil
}

// updateRegistryMetadata updates the registry.json file with the modified registry data
func updateRegistryMetadata(registry types.Registry, registryMetaFile, registryName string) error {
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry.json for '%s': %v", registryName, err)
	}
	if err := os.WriteFile(registryMetaFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry.json for '%s': %v", registryName, err)
	}
	return nil
}

// commitAndPushRemoval commits and pushes the removal changes to the remote repository
func commitAndPushRemoval(registriesDir, registryName, packageName, version string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}
	registryPath := filepath.Join(registriesDir, registryName)
	if err := os.Chdir(registryPath); err != nil {
		cleanupCommit(currentDir)
		return fmt.Errorf("failed to change to registry directory '%s': %v", registryPath, err)
	}
	addCmd := exec.Command("git", "add", ".")
	if addOutput, err := addCmd.CombinedOutput(); err != nil {
		cleanupCommit(currentDir)
		return fmt.Errorf("failed to stage registry changes: %v\nOutput: %s", err, addOutput)
	}
	commitMsg := fmt.Sprintf("Removed package '%s'", packageName)
	if version != "" {
		commitMsg = fmt.Sprintf("Removed version '%s' of package '%s'", version, packageName)
	}
	commitCmd := exec.Command("git", "commit", "-m", commitMsg)
	if commitOutput, err := commitCmd.CombinedOutput(); err != nil {
		cleanupCommit(currentDir)
		return fmt.Errorf("failed to commit registry changes: %v\nOutput: %s", err, commitOutput)
	}
	pushCmd := exec.Command("git", "push", "origin", "main")
	if pushOutput, err := pushCmd.CombinedOutput(); err != nil {
		cleanupCommit(currentDir)
		return fmt.Errorf("failed to push registry changes: %v\nOutput: %s", err, pushOutput)
	}
	if err := restoreCommitDir(currentDir); err != nil {
		return err
	}
	return nil
}
