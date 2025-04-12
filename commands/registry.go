package commands

import (
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
func RegistryStatus(cmd *cobra.Command, args []string) {
	registryName := validateStatusArgs(args, cmd)
	cosmDir := getCosmDir()
	registriesDir := setupRegistriesDir(cosmDir)
	assertRegistryExists(registriesDir, registryName)
	registry, _ := loadRegistryMetadata(registriesDir, registryName)
	printRegistryStatus(registryName, registry)
}

// RegistryInit initializes a new package registry
func RegistryInit(cmd *cobra.Command, args []string) {
	originalDir, registryName, gitURL, registriesDir := setupAndParseInitArgs(cmd, args)
	registryNames := loadAndCheckRegistries(registriesDir, registryName)
	registrySubDir := cloneAndEnterRegistry(registriesDir, registryName, gitURL, originalDir)
	ensureDirectoryEmpty(registrySubDir, gitURL, originalDir)
	updateRegistriesList(registriesDir, registryNames, registryName, originalDir, registrySubDir)
	initializeRegistryMetadata(registrySubDir, registryName, gitURL, originalDir)
	commitAndPushInitialRegistryChanges(registryName, gitURL, originalDir, registrySubDir)
	restoreOriginalDir(originalDir, registrySubDir)
	fmt.Printf("Initialized registry '%s' with Git URL: %s\n", registryName, gitURL)
}

// RegistryAdd adds a package version to a registry
func RegistryAdd(cmd *cobra.Command, args []string) {
	registryName, packageGitURL, cosmDir, registriesDir := parseArgsAndSetup(cmd, args)
	prepareRegistry(registriesDir, registryName)
	registry, registryMetaFile := loadRegistryMetadata(registriesDir, registryName)
	tmpClonePath := clonePackageToTempDir(cosmDir, packageGitURL)
	enterCloneDir(tmpClonePath)
	project := validateProjectFile(packageGitURL, tmpClonePath)
	ensurePackageNotRegistered(registry, project.Name, registryName, tmpClonePath)
	validTags := validateAndCollectVersionTags(packageGitURL, project.Version, tmpClonePath)
	packageDir := setupPackageDir(registriesDir, registryName, project.Name, tmpClonePath)
	updatePackageVersions(packageDir, project.Name, project.UUID, packageGitURL, validTags, project, tmpClonePath)
	finalizePackageAddition(cosmDir, tmpClonePath, project.UUID, registriesDir, registryName, project.Name, &registry, registryMetaFile, validTags[0])
	fmt.Printf("Added package '%s' with UUID '%s' to registry '%s'\n", project.Name, project.UUID, registryName)
}

// cleanupTempClone removes the temporary clone directory
func cleanupTempClone(tmpClonePath string) {
	if err := os.RemoveAll(tmpClonePath); err != nil {
		fmt.Printf("Warning: Failed to clean up temporary clone directory %s: %v\n", tmpClonePath, err)
	}
}

// clonePackageToTempDir creates a temp clone directly in the clones directory
func clonePackageToTempDir(cosmDir, packageGitURL string) string {
	clonesDir := filepath.Join(cosmDir, "clones")
	if err := os.MkdirAll(clonesDir, 0755); err != nil {
		fmt.Printf("Error creating clones directory: %v\n", err)
		os.Exit(1)
	}
	tmpClonePath := filepath.Join(clonesDir, "tmp-clone-"+uuid.New().String())

	if err := exec.Command("git", "clone", packageGitURL, tmpClonePath).Run(); err != nil {
		cloneOutput, _ := exec.Command("git", "clone", packageGitURL, tmpClonePath).CombinedOutput()
		fmt.Printf("Error cloning package repository at '%s': %v\nOutput: %s\n", packageGitURL, err, cloneOutput)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	return tmpClonePath
}

// moveCloneToPermanentDir moves the cloned directory to its permanent location, replacing any existing clone
func moveCloneToPermanentDir(cosmDir, tmpClonePath, packageUUID string) string {
	clonesDir := filepath.Join(cosmDir, "clones")
	packageClonePath := filepath.Join(clonesDir, packageUUID)

	// If the permanent clone directory already exists, remove it
	if _, err := os.Stat(packageClonePath); !os.IsNotExist(err) {
		if err := os.RemoveAll(packageClonePath); err != nil {
			fmt.Printf("Error removing existing clone at %s: %v\n", packageClonePath, err)
			cleanupTempClone(tmpClonePath)
			os.Exit(1)
		}
		fmt.Printf("Warning: Replaced existing clone for UUID '%s' at %s\n", packageUUID, packageClonePath)
	}

	// Move the temporary clone to the permanent location
	if err := os.Rename(tmpClonePath, packageClonePath); err != nil {
		fmt.Printf("Error moving package to %s: %v\n", packageClonePath, err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	return packageClonePath
}

// assertRegistryExists verifies that the specified registry exists in registries.json
func assertRegistryExists(registriesDir, registryName string) {
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if _, err := os.Stat(registriesFile); os.IsNotExist(err) {
		fmt.Println("Error: No registries found (run 'cosm registry init' first)")
		os.Exit(1)
	}
	var registryNames []string
	data, err := os.ReadFile(registriesFile)
	if err != nil {
		fmt.Printf("Error reading registries.json: %v\n", err)
		os.Exit(1)
	}
	if err := json.Unmarshal(data, &registryNames); err != nil {
		fmt.Printf("Error parsing registries.json: %v\n", err)
		os.Exit(1)
	}
	registryExists := false
	for _, name := range registryNames {
		if name == registryName {
			registryExists = true
			break
		}
	}
	if !registryExists {
		fmt.Printf("Error: Registry '%s' not found in registries.json\n", registryName)
		os.Exit(1)
	}
}

// pullRegistryUpdates pulls changes from the registry's remote Git repository
func pullRegistryUpdates(registriesDir, registryName string) {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	registryDir := filepath.Join(registriesDir, registryName)
	if err := os.Chdir(registryDir); err != nil {
		restoreDirBeforeExit(currentDir)
		fmt.Printf("Error changing to registry directory %s: %v\n", registryDir, err)
		os.Exit(1)
	}

	pullCmd := exec.Command("git", "pull", "origin", "main")
	pullOutput, err := pullCmd.CombinedOutput()
	if err != nil {
		restoreDirBeforeExit(currentDir)
		fmt.Printf("Error pulling updates from registry '%s': %v\nOutput: %s\n", registryName, err, pullOutput)
		os.Exit(1)
	}

	restoreDirBeforeExit(currentDir)
}

// loadRegistryMetadata loads and validates the registry metadata from registry.json
func loadRegistryMetadata(registriesDir, registryName string) (types.Registry, string) {
	registryMetaFile := filepath.Join(registriesDir, registryName, "registry.json")
	data, err := os.ReadFile(registryMetaFile)
	if err != nil {
		fmt.Printf("Error reading registry.json for '%s': %v\n", registryName, err)
		os.Exit(1)
	}
	var registry types.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		fmt.Printf("Error parsing registry.json for '%s': %v\n", registryName, err)
		os.Exit(1)
	}
	if registry.Packages == nil {
		registry.Packages = make(map[string]string)
	}
	return registry, registryMetaFile
}

// updateRegistryMetadata updates and writes the registry metadata to registry.json
func updateRegistryMetadata(registry *types.Registry, packageName, packageUUID, registryMetaFile string) {
	registry.Packages[packageName] = packageUUID
	data, err := json.MarshalIndent(*registry, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling registry.json: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(registryMetaFile, data, 0644); err != nil {
		fmt.Printf("Error writing registry.json: %v\n", err)
		os.Exit(1)
	}
}

// commitAndPushRegistryChanges commits and pushes changes to the registry's Git repository
func commitAndPushRegistryChanges(registriesDir, registryName, packageName, versionTag string) {
	registryDir := filepath.Join(registriesDir, registryName)
	if err := os.Chdir(registryDir); err != nil {
		fmt.Printf("Error changing to registry directory %s: %v\n", registryDir, err)
		os.Exit(1)
	}

	addCmd := exec.Command("git", "add", ".")
	addOutput, err := addCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error staging changes in registry: %v\nOutput: %s\n", err, addOutput)
		os.Exit(1)
	}

	commitMsg := fmt.Sprintf("Added package %s version %s", packageName, versionTag)
	commitCmd := exec.Command("git", "commit", "-m", commitMsg)
	commitOutput, err := commitCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error committing changes in registry: %v\nOutput: %s\n", err, commitOutput)
		os.Exit(1)
	}

	pushCmd := exec.Command("git", "push", "origin", "main")
	pushOutput, err := pushCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error pushing changes to registry: %v\nOutput: %s\n", err, pushOutput)
		os.Exit(1)
	}
}

// validateProjectFile reads and validates Project.json, returning the project
func validateProjectFile(packageGitURL, tmpClonePath string) types.Project {
	data, err := os.ReadFile("Project.json")
	if err != nil {
		fmt.Printf("Error: Repository at '%s' does not contain a Project.json file\n", packageGitURL)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		fmt.Printf("Error: Invalid Project.json in repository at '%s': %v\n", packageGitURL, err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	if project.Name == "" {
		fmt.Printf("Error: Project.json in repository at '%s' does not contain a valid package name\n", packageGitURL)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	if project.UUID == "" {
		fmt.Printf("Error: Project.json in repository at '%s' does not contain a valid UUID\n", packageGitURL)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	if _, err := uuid.Parse(project.UUID); err != nil {
		fmt.Printf("Error: Invalid UUID '%s' in Project.json at '%s': %v\n", project.UUID, packageGitURL, err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	if project.Version == "" {
		fmt.Printf("Error: Project.json at '%s' does not contain a version\n", packageGitURL)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	// Validate version parsing
	_ = parseSemVer(project.Version) // Will exit if invalid
	return project
}

// validateAndCollectVersionTags fetches Git tags, or releases the current version if none exist
func validateAndCollectVersionTags(packageGitURL string, packageVersion string, tmpClonePath string) []string {
	tagOutput, err := exec.Command("git", "tag").CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(tagOutput))) == 0 {
		// No tags found, use Project.json packageVersion and tag it
		if packageVersion == "" {
			fmt.Printf("Error: Project.json at '%s' has no version specified\n", packageGitURL)
			cleanupTempClone(tmpClonePath)
			os.Exit(1)
		}

		// Tag the current version
		if err := exec.Command("git", "tag", packageVersion).Run(); err != nil {
			fmt.Printf("Error tagging version '%s' in repository at '%s': %v\n", packageVersion, packageGitURL, err)
			cleanupTempClone(tmpClonePath)
			os.Exit(1)
		}
		// Push the tag to the remote
		if err := exec.Command("git", "push", "origin", packageVersion).Run(); err != nil {
			fmt.Printf("Error pushing tag '%s' to origin for repository at '%s': %v\n", packageVersion, packageGitURL, err)
			cleanupTempClone(tmpClonePath)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "No valid tags found; released version '%s' from Project.json to repository at '%s'\n", packageVersion, packageGitURL)
		return []string{packageVersion}
	}

	tags := strings.Split(strings.TrimSpace(string(tagOutput)), "\n")
	var validTags []string
	for _, tag := range tags {
		if strings.HasPrefix(tag, "v") && len(strings.Split(tag, ".")) >= 2 {
			validTags = append(validTags, tag)
		}
	}
	if len(validTags) == 0 {
		fmt.Printf("Error: No valid version tags (e.g., vX.Y.Z) found in repository at '%s'\n", packageGitURL)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	return validTags
}

// updateVersionsList loads and writes versions.json, updating with new tags
func updateVersionsList(packageDir string, tagsToAdd *[]string, tmpClonePath string) {
	versionsFile := filepath.Join(packageDir, "versions.json")
	var existingVersions []string
	if data, err := os.ReadFile(versionsFile); err == nil {
		if err := json.Unmarshal(data, &existingVersions); err != nil {
			fmt.Printf("Error parsing versions.json at %s: %v\n", versionsFile, err)
			cleanupTempClone(tmpClonePath)
			os.Exit(1)
		}
	}
	for _, versionTag := range *tagsToAdd {
		versionExists := false
		for _, v := range existingVersions {
			if v == versionTag {
				versionExists = true
				break
			}
		}
		if !versionExists {
			existingVersions = append(existingVersions, versionTag)
		}
	}
	data, err := json.MarshalIndent(existingVersions, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling versions.json: %v\n", err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	if err := os.WriteFile(versionsFile, data, 0644); err != nil {
		fmt.Printf("Error writing versions.json: %v\n", err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
}

// addPackageVersion adds a single version to the package directory
func addPackageVersion(packageDir, packageName, packageUUID, packageGitURL string, versionTag string, project types.Project, tmpClonePath string) {
	versionDir := filepath.Join(packageDir, versionTag)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		fmt.Printf("Error creating version directory %s: %v\n", versionDir, err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}

	sha1Output, err := exec.Command("git", "rev-list", "-n", "1", versionTag).Output()
	if err != nil {
		fmt.Printf("Error getting SHA1 for tag '%s': %v\n", versionTag, err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
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
		fmt.Printf("Error marshaling specs.json for version '%s': %v\n", versionTag, err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	specsFile := filepath.Join(versionDir, "specs.json")
	if err := os.WriteFile(specsFile, data, 0644); err != nil {
		fmt.Printf("Error writing specs.json for version '%s': %v\n", versionTag, err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
}

// setupAndParseInitArgs validates arguments and sets up directories for RegistryInit
func setupAndParseInitArgs(cmd *cobra.Command, args []string) (string, string, string, string) {
	originalDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting original directory: %v\n", err)
		os.Exit(1)
	}

	if len(args) != 2 {
		fmt.Println("Error: Exactly two arguments required (e.g., cosm registry init <registry name> <giturl>)")
		cmd.Usage()
		os.Exit(1)
	}
	registryName := args[0]
	gitURL := args[1]

	if registryName == "" {
		fmt.Println("Error: Registry name cannot be empty")
		os.Exit(1)
	}
	if gitURL == "" {
		fmt.Println("Error: Git URL cannot be empty")
		os.Exit(1)
	}

	cosmDir, err := getGlobalCosmDir()
	if err != nil {
		fmt.Printf("Error getting global .cosm directory: %v\n", err)
		os.Exit(1)
	}
	registriesDir := filepath.Join(cosmDir, "registries")
	if err := os.MkdirAll(registriesDir, 0755); err != nil {
		fmt.Printf("Error creating %s directory: %v\n", registriesDir, err)
		os.Exit(1)
	}

	return originalDir, registryName, gitURL, registriesDir
}

// loadAndCheckRegistries loads registries.json and checks for duplicate registry names
func loadAndCheckRegistries(registriesDir, registryName string) []string {
	registriesFile := filepath.Join(registriesDir, "registries.json")
	var registryNames []string
	if data, err := os.ReadFile(registriesFile); err == nil {
		if err := json.Unmarshal(data, &registryNames); err != nil {
			fmt.Printf("Error parsing registries.json: %v\n", err)
			os.Exit(1)
		}
	} else if !os.IsNotExist(err) {
		fmt.Printf("Error reading registries.json: %v\n", err)
		os.Exit(1)
	}

	for _, name := range registryNames {
		if name == registryName {
			fmt.Printf("Error: Registry '%s' already exists\n", registryName)
			os.Exit(1)
		}
	}

	return registryNames
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
func cloneAndEnterRegistry(registriesDir, registryName, gitURL, originalDir string) string {
	registrySubDir := filepath.Join(registriesDir, registryName)
	cloneCmd := exec.Command("git", "clone", gitURL, registrySubDir)
	cloneOutput, err := cloneCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error cloning repository at '%s' into %s: %v\nOutput: %s\n", gitURL, registrySubDir, err, cloneOutput)
		os.Exit(1) // No cleanup needed yet as registrySubDir isn’t created
	}

	// Change to the cloned directory
	if err := os.Chdir(registrySubDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error changing to registry directory %s: %v\n", registrySubDir, err)
		cleanupInit(originalDir, registrySubDir, true)
		os.Exit(1)
	}
	return registrySubDir
}

// ensureDirectoryEmpty checks if the cloned directory is empty except for .git
func ensureDirectoryEmpty(dir, gitURL, originalDir string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading directory %s: %v\n", dir, err)
		cleanupInit(originalDir, dir, true)
		os.Exit(1)
	}
	for _, file := range files {
		if file.Name() != ".git" { // Ignore .git directory
			fmt.Fprintf(os.Stderr, "Error: Repository at '%s' cloned into %s is not empty (contains %s)\n", gitURL, dir, file.Name())
			cleanupInit(originalDir, dir, true)
			os.Exit(1)
		}
	}
}

// updateRegistriesList adds the registry name to registries.json
func updateRegistriesList(registriesDir string, registryNames []string, registryName, originalDir, registrySubDir string) {
	registryNames = append(registryNames, registryName)
	data, err := json.MarshalIndent(registryNames, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling registries.json: %v\n", err)
		cleanupInit(originalDir, registrySubDir, true)
		os.Exit(1)
	}
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if err := os.WriteFile(registriesFile, data, 0644); err != nil {
		fmt.Printf("Error writing registries.json: %v\n", err)
		cleanupInit(originalDir, registrySubDir, true)
		os.Exit(1)
	}
}

// initializeRegistryMetadata creates and writes the registry.json file
func initializeRegistryMetadata(registrySubDir, registryName, gitURL, originalDir string) string {
	registryMetaFile := filepath.Join(registrySubDir, "registry.json")
	registry := types.Registry{
		Name:     registryName,
		UUID:     uuid.New().String(),
		GitURL:   gitURL,
		Packages: make(map[string]string),
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling registry.json: %v\n", err)
		cleanupInit(originalDir, registrySubDir, true)
		os.Exit(1)
	}
	if err := os.WriteFile(registryMetaFile, data, 0644); err != nil {
		fmt.Printf("Error writing registry.json: %v\n", err)
		cleanupInit(originalDir, registrySubDir, true)
		os.Exit(1)
	}
	return registryMetaFile
}

// commitAndPushInitialRegistryChanges stages, commits, and pushes the initial registry changes
func commitAndPushInitialRegistryChanges(registryName, gitURL, originalDir, registrySubDir string) {
	addCmd := exec.Command("git", "add", "registry.json")
	addOutput, err := addCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error staging registry.json: %v\nOutput: %s\n", err, addOutput)
		cleanupInit(originalDir, registrySubDir, true)
		os.Exit(1)
	}
	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("Initialized registry %s", registryName))
	commitOutput, err := commitCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error committing initial registry setup: %v\nOutput: %s\n", err, commitOutput)
		cleanupInit(originalDir, registrySubDir, true)
		os.Exit(1)
	}
	pushCmd := exec.Command("git", "push", "origin", "main")
	pushOutput, err := pushCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error pushing initial commit to %s: %v\nOutput: %s\n", gitURL, err, pushOutput)
		cleanupInit(originalDir, registrySubDir, true)
		os.Exit(1)
	}
}

// restoreOriginalDir returns to the original directory without removing the registry subdir
func restoreOriginalDir(originalDir, registrySubDir string) {
	if err := os.Chdir(originalDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error returning to original directory %s: %v\n", originalDir, err)
		cleanupInit(originalDir, registrySubDir, true)
		os.Exit(1)
	}
}

// parseArgsAndSetup validates arguments and sets up directories for RegistryAdd
func parseArgsAndSetup(cmd *cobra.Command, args []string) (string, string, string, string) {
	if len(args) != 2 {
		fmt.Println("Error: Exactly two arguments required (e.g., cosm registry add <registry_name> <package giturl>)")
		cmd.Usage()
		os.Exit(1)
	}
	registryName := args[0]
	packageGitURL := args[1]

	if registryName == "" {
		fmt.Println("Error: Registry name cannot be empty")
		os.Exit(1)
	}
	if packageGitURL == "" {
		fmt.Println("Error: Package Git URL cannot be empty")
		os.Exit(1)
	}

	cosmDir, err := getGlobalCosmDir()
	if err != nil {
		fmt.Printf("Error getting global .cosm directory: %v\n", err)
		os.Exit(1)
	}
	registriesDir := filepath.Join(cosmDir, "registries")

	return registryName, packageGitURL, cosmDir, registriesDir
}

// prepareRegistry ensures the registry exists and is up-to-date
func prepareRegistry(registriesDir, registryName string) {
	assertRegistryExists(registriesDir, registryName)
	pullRegistryUpdates(registriesDir, registryName)
}

// enterCloneDir changes to the temporary clone directory
func enterCloneDir(tmpClonePath string) {
	if err := os.Chdir(tmpClonePath); err != nil {
		fmt.Printf("Error changing to cloned directory: %v\n", err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
}

// ensurePackageNotRegistered checks if the package is already in the registry
func ensurePackageNotRegistered(registry types.Registry, packageName, registryName, tmpClonePath string) {
	if _, exists := registry.Packages[packageName]; exists {
		fmt.Fprintf(os.Stderr, "Error: Package '%s' is already registered in registry '%s'\n", packageName, registryName)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
}

// setupPackageDir creates the package directory structure
func setupPackageDir(registriesDir, registryName, packageName, tmpClonePath string) string {
	packageFirstLetter := strings.ToUpper(string(packageName[0]))
	packageDir := filepath.Join(registriesDir, registryName, packageFirstLetter, packageName)
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		fmt.Printf("Error creating package directory %s: %v\n", packageDir, err)
		cleanupTempClone(tmpClonePath)
		os.Exit(1)
	}
	return packageDir
}

// updatePackageVersions updates the versions list and adds version specs
func updatePackageVersions(packageDir, packageName, packageUUID, packageGitURL string, validTags []string, project types.Project, tmpClonePath string) {
	updateVersionsList(packageDir, &validTags, tmpClonePath)
	for _, versionTag := range validTags {
		addPackageVersion(packageDir, packageName, packageUUID, packageGitURL, versionTag, project, tmpClonePath)
	}
}

// finalizePackageAddition completes the package addition process
func finalizePackageAddition(cosmDir, tmpClonePath, packageUUID, registriesDir, registryName, packageName string, registry *types.Registry, registryMetaFile string, firstVersionTag string) {
	moveCloneToPermanentDir(cosmDir, tmpClonePath, packageUUID)
	updateRegistryMetadata(registry, packageName, packageUUID, registryMetaFile)
	commitAndPushRegistryChanges(registriesDir, registryName, packageName, firstVersionTag)
}

// validateStatusArgs checks the command-line arguments for validity
func validateStatusArgs(args []string, cmd *cobra.Command) string {
	if len(args) != 1 {
		fmt.Println("Error: Exactly one argument required (e.g., cosm registry status <registry_name>)")
		cmd.Usage()
		os.Exit(1)
	}
	registryName := args[0]
	if registryName == "" {
		fmt.Println("Error: Registry name cannot be empty")
		os.Exit(1)
	}
	return registryName
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

func RegistryClone(cmd *cobra.Command, args []string) {
}

func RegistryDelete(cmd *cobra.Command, args []string) {
}

func RegistryUpdate(cmd *cobra.Command, args []string) {
}

func RegistryRm(cmd *cobra.Command, args []string) {
}
