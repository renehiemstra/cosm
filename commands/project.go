package commands

import (
	"bufio"
	"cosm/types"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// Status displays the current cosmic status
func Status(cmd *cobra.Command, args []string) {
}

// Activate activates the current project if cosm.json exists
func Activate(cmd *cobra.Command, args []string) {
}

// Init initializes a new project with a Project.json file
func Init(cmd *cobra.Command, args []string) error {
	packageName, version := validateInitArgs(args, cmd)
	if packageName == "" {
		return fmt.Errorf("invalid arguments")
	}
	language := getInitLanguageFlag(cmd)
	if version != "" {
		if err := validateVersion(version); err != nil {
			return err
		}
	}
	projectUUID := uuid.New().String()
	authors, err := getGitAuthors()
	if err != nil {
		return err
	}
	if err := ensureProjectFileDoesNotExist("Project.json"); err != nil {
		return err
	}
	project := createProject(packageName, projectUUID, authors, language, version)
	data, err := marshalProject(project)
	if err != nil {
		return err
	}
	if err := writeProjectFile("Project.json", data); err != nil {
		return err
	}
	fmt.Printf("Initialized project '%s' with version %s and UUID %s\n", packageName, version, projectUUID)
	return nil
}

// Add adds a dependency to the project's Project.json file
func Add(cmd *cobra.Command, args []string) error { // Changed to return error
	packageName, versionTag, err := parseAddArgs(args) // Updated to handle error
	if err != nil {
		return err
	}
	project, err := loadProject("Project.json") // Updated to handle error
	if err != nil {
		return err
	}
	cosmDir, err := getCosmDir() // Updated to handle error
	if err != nil {
		return err
	}
	registryNames, err := loadRegistryNames(cosmDir) // Updated to handle error
	if err != nil {
		return err
	}
	selectedPackage, err := findPackageInRegistries(packageName, versionTag, cosmDir, registryNames) // Updated to handle error
	if err != nil {
		return err
	}
	if err := updateProjectWithDependency(project, packageName, versionTag, selectedPackage.RegistryName); err != nil { // Updated to handle error
		return err
	}
	return nil
}

func Rm(cmd *cobra.Command, args []string) {
}

// Release updates the project version and publishes it to the remote repository and registries
func Release(cmd *cobra.Command, args []string) error {
	project, err := loadProject("Project.json") // Fixed to handle two return values
	if err != nil {
		return err
	}
	ensureNoUncommittedChanges()
	ensureLocalRepoInSyncWithOrigin()
	newVersion, err := determineNewVersion(cmd, args, project.Version)
	if err != nil {
		return err
	}
	if err := validateNewVersion(newVersion, project.Version); err != nil {
		return err
	}
	ensureTagDoesNotExist(newVersion)
	registryName, _ := cmd.Flags().GetString("registry")
	registries := findHostingRegistries(project.Name, registryName)
	ensureRegistriesExist(registries, registryName)
	if err := updateProjectVersion(project, newVersion); err != nil {
		return err
	}
	publishToGitRemote(newVersion)
	publishToRegistries(project, registries, newVersion, getWorkingDir())
	fmt.Printf("Released version '%s' for project '%s'\n", newVersion, project.Name)
	return nil
}

func getWorkingDir() string {
	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting project directory: %v\n", err)
		os.Exit(1)
	}
	return projectDir
}

// validateInitArgs checks the command-line arguments for validity
func validateInitArgs(args []string, cmd *cobra.Command) (string, string) {
	if len(args) < 1 || len(args) > 2 {
		fmt.Println("Error: One or two arguments required (e.g., cosm init <package-name> [version])")
		cmd.Usage()
		os.Exit(1)
	}
	packageName := args[0]
	if packageName == "" {
		fmt.Println("Error: Package name cannot be empty")
		os.Exit(1)
	}

	// Check version from args or flag
	version := ""
	if len(args) == 2 {
		version = args[1]
	}
	flagVersion, _ := cmd.Flags().GetString("version")
	if version != "" && flagVersion != "" {
		fmt.Println("Error: Cannot specify version both as an argument and a flag")
		cmd.Usage()
		os.Exit(1)
	}
	if version == "" {
		version = flagVersion
	}
	if version == "" {
		version = "v0.1.0" // Default version
	}
	return packageName, version
}

// getInitLanguageFlag retrieves the language flag from the command
func getInitLanguageFlag(cmd *cobra.Command) string {
	language, _ := cmd.Flags().GetString("language")
	return language
}

// validateVersion ensures the version starts with 'v'
func validateVersion(version string) error {
	if len(version) == 0 || version[0] != 'v' {
		return fmt.Errorf("version '%s' must start with 'v'", version)
	}
	return nil
}

// getGitAuthors retrieves the author info from git config or uses a default
func getGitAuthors() ([]string, error) {
	name, errName := exec.Command("git", "config", "user.name").Output()
	email, errEmail := exec.Command("git", "config", "user.email").Output()
	if errName != nil || errEmail != nil || len(name) == 0 || len(email) == 0 {
		fmt.Println("Warning: Could not retrieve git user.name or user.email, defaulting to '[unknown]unknown@author.com'")
		return []string{"[unknown]unknown@author.com"}, nil // Return default with no error
	}
	gitName := strings.TrimSpace(string(name))
	gitEmail := strings.TrimSpace(string(email))
	return []string{fmt.Sprintf("[%s]%s", gitName, gitEmail)}, nil
}

// ensureProjectFileDoesNotExist checks if Project.json already exists
func ensureProjectFileDoesNotExist(projectFile string) error {
	if _, err := os.Stat(projectFile); !os.IsNotExist(err) {
		return fmt.Errorf("Project.json already exists in this directory")
	}
	return nil
}

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

// marshalProject converts the project struct to JSON
func marshalProject(project types.Project) ([]byte, error) { // Changed to return ([]byte, error)
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Project.json: %v", err) // Return error
	}
	return data, nil
}

// writeProjectFile writes the project data to Project.json
func writeProjectFile(projectFile string, data []byte) error { // Changed to return error
	if err := os.WriteFile(projectFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write Project.json: %v", err) // Return error
	}
	return nil
}

// parseAddArgs validates and parses the package_name@version argument
func parseAddArgs(args []string) (string, string, error) { // Changed to return error
	if len(args) != 1 {
		return "", "", fmt.Errorf("exactly one argument required in the format <package_name>@v<version_number> (e.g., cosm add mypkg@v1.2.3)")
	}
	depArg := args[0]
	parts := strings.SplitN(depArg, "@", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("argument '%s' must be in the format <package_name>@v<version_number>", depArg)
	}
	packageName, versionTag := parts[0], parts[1]
	if packageName == "" {
		return "", "", fmt.Errorf("package name cannot be empty")
	}
	if !strings.HasPrefix(versionTag, "v") {
		return "", "", fmt.Errorf("version '%s' must start with 'v'", versionTag)
	}
	return packageName, versionTag, nil
}

// loadProject reads and parses the Project.json file
func loadProject(projectFile string) (*types.Project, error) { // Changed to return (*types.Project, error)
	if _, err := os.Stat(projectFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no Project.json found in current directory")
	}
	data, err := os.ReadFile(projectFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read Project.json: %v", err)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("failed to parse Project.json: %v", err)
	}
	if project.Deps == nil {
		project.Deps = make(map[string]string)
	}
	return &project, nil
}

// getCosmDir retrieves the global .cosm directory
func getCosmDir() (string, error) { // Changed to return (string, error)
	cosmDir, err := getGlobalCosmDir()
	if err != nil {
		return "", fmt.Errorf("failed to get global .cosm directory: %v", err)
	}
	return cosmDir, nil
}

// loadRegistryNames loads the list of registry names from registries.json
func loadRegistryNames(cosmDir string) ([]string, error) { // Changed to return ([]string, error)
	registriesDir := filepath.Join(cosmDir, "registries")
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if _, err := os.Stat(registriesFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no registries found (run 'cosm registry init' first)")
	}
	data, err := os.ReadFile(registriesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read registries.json: %v", err)
	}
	var registryNames []string
	if err := json.Unmarshal(data, &registryNames); err != nil {
		return nil, fmt.Errorf("failed to parse registries.json: %v", err)
	}
	if len(registryNames) == 0 {
		return nil, fmt.Errorf("no registries available to search for packages")
	}
	return registryNames, nil
}

// packageLocation represents a package found in a registry
type packageLocation struct {
	RegistryName string
	Specs        types.Specs
}

// restoreDirBeforeExit restores the working directory and exits if restoration fails
func restoreDirBeforeExit(currentDir string) {
	if err := os.Chdir(currentDir); err != nil {
		fmt.Printf("Warning: Failed to restore directory to %s: %v\n", currentDir, err)
		os.Exit(1)
	}
}

// findPackageInRegistries searches for a package across all registries
func findPackageInRegistries(packageName, versionTag, cosmDir string, registryNames []string) (packageLocation, error) { // Changed to return (packageLocation, error)
	var foundPackages []packageLocation
	registriesDir := filepath.Join(cosmDir, "registries")

	for _, regName := range registryNames {
		if pkg, found := findPackageInRegistry(packageName, versionTag, registriesDir, regName); found {
			foundPackages = append(foundPackages, pkg)
		}
	}

	return selectPackageFromResults(packageName, versionTag, foundPackages)
}

// findPackageInRegistry searches for a package in a single registry
func findPackageInRegistry(packageName, versionTag, registriesDir, registryName string) (packageLocation, bool) {
	// Load registry metadata after pull
	pullRegistryUpdates(registriesDir, registryName)
	registry, _ := loadRegistryMetadata(registriesDir, registryName)

	if _, exists := registry.Packages[packageName]; !exists {
		return packageLocation{}, false
	}

	specsFile := filepath.Join(registriesDir, registryName, strings.ToUpper(string(packageName[0])), packageName, versionTag, "specs.json")
	if _, err := os.Stat(specsFile); os.IsNotExist(err) {
		return packageLocation{}, false
	}
	data, err := os.ReadFile(specsFile)
	if err != nil {
		fmt.Printf("Error reading specs.json for '%s' in registry '%s': %v\n", packageName, registryName, err)
		os.Exit(1)
	}
	var specs types.Specs
	if err := json.Unmarshal(data, &specs); err != nil {
		fmt.Printf("Error parsing specs.json for '%s' in registry '%s': %v\n", packageName, registryName, err)
		os.Exit(1)
	}
	if specs.Version != versionTag {
		return packageLocation{}, false
	}
	return packageLocation{RegistryName: registryName, Specs: specs}, true
}

// selectPackageFromResults handles the selection of a package from multiple matches
func selectPackageFromResults(packageName, versionTag string, foundPackages []packageLocation) (packageLocation, error) { // Changed to return (packageLocation, error)
	if len(foundPackages) == 0 {
		return packageLocation{}, fmt.Errorf("package '%s' with version '%s' not found in any registry", packageName, versionTag)
	}
	if len(foundPackages) == 1 {
		return foundPackages[0], nil
	}
	return promptUserForRegistry(packageName, versionTag, foundPackages)
}

// promptUserForRegistry handles multiple registry matches by prompting the user
func promptUserForRegistry(packageName, versionTag string, foundPackages []packageLocation) (packageLocation, error) { // Changed to return (packageLocation, error)
	fmt.Printf("Package '%s' v%s found in multiple registries:\n", packageName, versionTag)
	for i, pkg := range foundPackages {
		fmt.Printf("  %d. %s (Git URL: %s)\n", i+1, pkg.RegistryName, pkg.Specs.GitURL)
	}
	fmt.Printf("Please select a registry (enter number 1-%d): ", len(foundPackages))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())
	choiceNum := 0
	_, err := fmt.Sscanf(choice, "%d", &choiceNum)
	if err != nil || choiceNum < 1 || choiceNum > len(foundPackages) {
		return packageLocation{}, fmt.Errorf("invalid selection '%s': must be a number between 1 and %d", choice, len(foundPackages))
	}
	return foundPackages[choiceNum-1], nil
}

// updateProjectWithDependency adds the dependency and saves the updated project
func updateProjectWithDependency(project *types.Project, packageName, versionTag string, registryName string) error { // Changed to return error
	if _, exists := project.Deps[packageName]; exists {
		return fmt.Errorf("dependency '%s' already exists in project", packageName)
	}
	project.Deps[packageName] = versionTag

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Project.json: %v", err)
	}
	if err := os.WriteFile("Project.json", data, 0644); err != nil {
		return fmt.Errorf("failed to write Project.json: %v", err)
	}
	fmt.Printf("Added dependency '%s' %s from registry '%s' to project\n", packageName, versionTag, registryName)
	return nil
}

// ensureRegistriesExist checks if any registries are available for the release
func ensureRegistriesExist(registries []registryInfo, specificRegistry string) {
	if len(registries) == 0 {
		if specificRegistry != "" {
			fmt.Printf("Error: No registry named '%s' hosts package\n", specificRegistry)
		} else {
			fmt.Println("Error: No registries found hosting this package")
		}
		os.Exit(1)
	}
}

// ensureNoUncommittedChanges checks for uncommitted changes in the Git repo
func ensureNoUncommittedChanges() {
	statusCmd := exec.Command("git", "status", "--porcelain")
	output, err := statusCmd.Output()
	if err != nil {
		fmt.Printf("Error checking Git status: %v\n", err)
		os.Exit(1)
	}
	if len(strings.TrimSpace(string(output))) > 0 {
		fmt.Println("Error: Repository has uncommitted changes. Please commit or stash them before releasing.")
		os.Exit(1)
	}
}

// ensureLocalRepoInSyncWithOrigin ensures the local repo is ahead or in sync with origin
func ensureLocalRepoInSyncWithOrigin() {
	fetchCmd := exec.Command("git", "fetch", "origin")
	if err := fetchCmd.Run(); err != nil {
		fmt.Printf("Error fetching from origin: %v\n", err)
		os.Exit(1)
	}
	// Check if local is behind origin
	revListCmd := exec.Command("git", "rev-list", "--count", "HEAD..origin/main")
	output, err := revListCmd.Output()
	if err != nil {
		fmt.Printf("Error checking sync with origin: %v\n", err)
		os.Exit(1)
	}
	behindCount, _ := strconv.Atoi(strings.TrimSpace(string(output)))
	if behindCount > 0 {
		fmt.Println("Error: Local repository is behind origin. Please pull changes before releasing.")
		os.Exit(1)
	}
}

// determineNewVersion calculates the new version based on args or flags
func determineNewVersion(cmd *cobra.Command, args []string, currentVersion string) (string, error) { // Fixed to include currentVersion
	if len(args) == 1 {
		return args[0], nil
	}
	if len(args) > 1 {
		return "", fmt.Errorf("too many arguments: use 'cosm release v<version>' or a version flag (--patch, --minor, --major)")
	}

	patch, _ := cmd.Flags().GetBool("patch")
	minor, _ := cmd.Flags().GetBool("minor")
	major, _ := cmd.Flags().GetBool("major")
	count := 0
	if patch {
		count++
	}
	if minor {
		count++
	}
	if major {
		count++
	}
	if count > 1 {
		return "", fmt.Errorf("only one of --patch, --minor, or --major can be specified")
	}
	if count == 0 {
		return "", fmt.Errorf("specify a version (e.g., v1.2.3) or use --patch, --minor, or --major")
	}

	currentSemVer, err := parseSemVer(currentVersion) // Fixed to handle two return values
	if err != nil {
		return "", fmt.Errorf("failed to parse current version: %v", err)
	}
	switch {
	case patch:
		return fmt.Sprintf("v%d.%d.%d", currentSemVer.Major, currentSemVer.Minor, currentSemVer.Patch+1), nil
	case minor:
		return fmt.Sprintf("v%d.%d.0", currentSemVer.Major, currentSemVer.Minor+1), nil
	case major:
		return fmt.Sprintf("v%d.0.0", currentSemVer.Major+1), nil
	}
	return "", fmt.Errorf("internal error: no version increment selected") // Unreachable but added for safety
}

// semVer represents a semantic version (vX.Y.Z)
type semVer struct {
	Major, Minor, Patch int
}

func parseSemVer(version string) (semVer, error) {
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) < 2 {
		return semVer{}, fmt.Errorf("invalid version format '%s': must be vX.Y.Z or vX.Y", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semVer{}, fmt.Errorf("invalid major version in '%s': %v", version, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semVer{}, fmt.Errorf("invalid minor version in '%s': %v", version, err)
	}
	patch := 0
	if len(parts) > 2 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return semVer{}, fmt.Errorf("invalid patch version in '%s': %v", version, err)
		}
	}
	return semVer{Major: major, Minor: minor, Patch: patch}, nil
}

// validateNewVersion ensures the new version is valid and greater than the current
func validateNewVersion(newVersion, currentVersion string) error {
	if !strings.HasPrefix(newVersion, "v") {
		return fmt.Errorf("new version '%s' must start with 'v'", newVersion)
	}
	newSemVer, err := parseSemVer(newVersion)
	if err != nil {
		return err
	}
	currentSemVer, err := parseSemVer(currentVersion)
	if err != nil {
		return err
	}
	if newSemVer.Major < currentSemVer.Major ||
		(newSemVer.Major == currentSemVer.Major && newSemVer.Minor < currentSemVer.Minor) ||
		(newSemVer.Major == currentSemVer.Major && newSemVer.Minor == currentSemVer.Minor && newSemVer.Patch <= currentSemVer.Patch) {
		return fmt.Errorf("new version '%s' must be greater than current version '%s'", newVersion, currentVersion)
	}
	return nil
}

// ensureTagDoesNotExist checks if the new version tag already exists in the repo
func ensureTagDoesNotExist(newVersion string) {
	tagsCmd := exec.Command("git", "tag")
	output, err := tagsCmd.Output()
	if err != nil {
		fmt.Printf("Error listing Git tags: %v\n", err)
		os.Exit(1)
	}
	tags := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, tag := range tags {
		if tag == newVersion {
			fmt.Printf("Error: Tag '%s' already exists in the repository\n", newVersion)
			os.Exit(1)
		}
	}
}

// registryInfo holds registry details for release
type registryInfo struct {
	Name       string
	MetaFile   string
	PackageDir string
}

// findHostingRegistries identifies registries hosting the package
func findHostingRegistries(packageName, specificRegistry string) []registryInfo {
	cosmDir, err := getCosmDir() // Fixed to handle two return values
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	registryNames, err := loadRegistryNames(cosmDir) // Fixed to handle two return values
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	registriesDir := filepath.Join(cosmDir, "registries")
	var registries []registryInfo

	for _, regName := range registryNames {
		if specificRegistry != "" && regName != specificRegistry {
			continue
		}
		registry, _ := loadRegistryMetadata(registriesDir, regName)
		if _, exists := registry.Packages[packageName]; exists {
			packageDir := filepath.Join(registriesDir, regName, strings.ToUpper(string(packageName[0])), packageName)
			registries = append(registries, registryInfo{
				Name:       regName,
				MetaFile:   filepath.Join(registriesDir, regName, "registry.json"),
				PackageDir: packageDir,
			})
		}
	}
	return registries
}

// updateProjectVersion updates the version in Project.json and saves it
func updateProjectVersion(project *types.Project, newVersion string) error { // Changed to return error
	project.Version = newVersion
	data, err := marshalProject(*project) // Fixed to handle two return values
	if err != nil {
		return err
	}
	if err := writeProjectFile("Project.json", data); err != nil { // Updated to handle error
		return err
	}
	return nil
}

// publishToGitRemote commits, tags, and pushes the new version to the remote
func publishToGitRemote(newVersion string) {
	if err := exec.Command("git", "add", "Project.json").Run(); err != nil {
		fmt.Printf("Error staging Project.json: %v\n", err)
		os.Exit(1)
	}
	commitMsg := fmt.Sprintf("Release %s", newVersion)
	if err := exec.Command("git", "commit", "-m", commitMsg).Run(); err != nil {
		fmt.Printf("Error committing release: %v\n", err)
		os.Exit(1)
	}
	if err := exec.Command("git", "tag", newVersion).Run(); err != nil {
		fmt.Printf("Error tagging release '%s': %v\n", newVersion, err)
		os.Exit(1)
	}
	if err := exec.Command("git", "push", "origin", "main").Run(); err != nil {
		fmt.Printf("Error pushing to origin/main: %v\n", err)
		os.Exit(1)
	}
	if err := exec.Command("git", "push", "origin", newVersion).Run(); err != nil {
		fmt.Printf("Error pushing tag '%s' to origin: %v\n", newVersion, err)
		os.Exit(1)
	}
}

// publishToRegistries adds the new release to the specified registries
func publishToRegistries(project *types.Project, registries []registryInfo, newVersion string, projectDir string) {
	cosmDir, err := getCosmDir() // Fixed to handle two return values
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	registryDir := setupRegistriesDir(cosmDir) // Fixed to use cosmDir
	for _, reg := range registries {
		pullRegistryUpdates(registryDir, reg.Name)
		updateRegistryVersions(reg.PackageDir, newVersion, project, reg.Name, projectDir)
		commitAndPushRegistryChanges(registryDir, reg.Name, project.Name, newVersion)
	}
}

// updateRegistryVersions updates versions.json and adds a specs file for the new version
func updateRegistryVersions(packageDir, newVersion string, project *types.Project, registryName, projectDir string) {
	versionsFile := filepath.Join(packageDir, "versions.json")
	var versions []string
	if data, err := os.ReadFile(versionsFile); err == nil {
		if err := json.Unmarshal(data, &versions); err != nil {
			fmt.Printf("Error parsing versions.json in registry '%s': %v\n", registryName, err)
			os.Exit(1)
		}
	}
	for _, v := range versions {
		if v == newVersion {
			fmt.Printf("Error: Version '%s' already exists in registry '%s'\n", newVersion, registryName)
			os.Exit(1)
		}
	}
	versions = append(versions, newVersion)
	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling versions.json in registry '%s': %v\n", registryName, err)
		os.Exit(1)
	}
	if err := os.WriteFile(versionsFile, data, 0644); err != nil {
		fmt.Printf("Error writing versions.json in registry '%s': %v\n", registryName, err)
		os.Exit(1)
	}

	versionDir := filepath.Join(packageDir, newVersion)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		fmt.Printf("Error creating version directory '%s' in registry '%s': %v\n", versionDir, registryName, err)
		os.Exit(1)
	}
	sha1 := getSHA1ForTag(newVersion, projectDir, fmt.Sprintf("registry '%s'", registryName))

	specs := types.Specs{
		Name:    project.Name,
		UUID:    project.UUID,
		Version: newVersion,
		GitURL:  "", // Could fetch from git config if needed
		SHA1:    sha1,
		Deps:    project.Deps,
	}
	data, err = json.MarshalIndent(specs, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling specs.json for version '%s' in registry '%s': %v\n", newVersion, registryName, err)
		os.Exit(1)
	}
	specsFile := filepath.Join(versionDir, "specs.json")
	if err := os.WriteFile(specsFile, data, 0644); err != nil {
		fmt.Printf("Error writing specs.json for version '%s' in registry '%s': %v\n", newVersion, registryName, err)
		os.Exit(1)
	}
}

// getSHA1ForTag retrieves the SHA1 hash for a given tag in the specified directory
func getSHA1ForTag(tag, dir, context string) string {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Printf("Error changing to directory %s: %v\n", dir, err)
		os.Exit(1)
	}
	sha1Output, err := exec.Command("git", "rev-list", "-n", "1", tag).Output()
	if err != nil {
		fmt.Printf("Error getting SHA1 for tag '%s' in %s: %v\n", tag, context, err)
		os.Chdir(currentDir) // Restore directory before exiting
		os.Exit(1)
	}
	if err := os.Chdir(currentDir); err != nil {
		fmt.Printf("Error restoring directory to %s: %v\n", currentDir, err)
		os.Exit(1)
	}
	return strings.TrimSpace(string(sha1Output))
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

// checkoutVersion switches the clone to the specified SHA1
func checkoutVersion(clonePath, sha1 string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(currentDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to restore directory to %s: %v\n", currentDir, err)
		}
	}()

	if err := os.Chdir(clonePath); err != nil {
		return fmt.Errorf("failed to change to clone directory %s: %v", clonePath, err)
	}

	// Fetch updates to ensure we have the latest refs
	if err := exec.Command("git", "fetch", "origin").Run(); err != nil {
		return fmt.Errorf("failed to fetch updates: %v", err)
	}

	// Checkout the specific SHA1
	cmd := exec.Command("git", "checkout", sha1)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout SHA1 %s: %v\nOutput: %s", sha1, err, output)
	}

	return nil
}

// revertClone returns the clone to its previous branch or state using 'git checkout -'
func revertClone(clonePath string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(currentDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to restore directory to %s: %v\n", currentDir, err)
		}
	}()

	if err := os.Chdir(clonePath); err != nil {
		return fmt.Errorf("failed to change to clone directory %s: %v", clonePath, err)
	}

	// Revert to the previous branch or commit state
	cmd := exec.Command("git", "checkout", "-")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to revert clone to previous state: %v\nOutput: %s", err, output)
	}

	return nil
}

// copyFile copies a single file from src to dest using io.Copy
func copyFile(src, dest string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %v", src, err)
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %v", dest, err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %v", src, dest, err)
	}

	// Ensure the destination file has the same permissions as the source
	if err := destFile.Chmod(mode); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %v", dest, err)
	}

	return nil
}

func Develop(cmd *cobra.Command, args []string) {

}

func Free(cmd *cobra.Command, args []string) {

}

func Upgrade(cmd *cobra.Command, args []string) {

}

func Downgrade(cmd *cobra.Command, args []string) {

}
