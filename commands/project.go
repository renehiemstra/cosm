package commands

import (
	"bufio"
	"cosm/types"
	"encoding/json"
	"fmt"
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
func Init(cmd *cobra.Command, args []string) {
	packageName := validateInitArgs(args, cmd)
	language, version := getInitFlags(cmd)
	validateVersion(version)
	projectUUID := uuid.New().String()
	authors := getGitAuthors()
	ensureProjectFileDoesNotExist("Project.json")
	project := createProject(packageName, projectUUID, authors, language, version)
	data := marshalProject(project)
	writeProjectFile("Project.json", data)
	fmt.Printf("Initialized project '%s' with version %s and UUID %s\n", packageName, version, projectUUID)
}

// Add adds a dependency to the project's Project.json file
func Add(cmd *cobra.Command, args []string) {
	packageName, versionTag := parseAddArgs(args)
	project := loadProject("Project.json")
	cosmDir := getCosmDir()
	registryNames := loadRegistryNames(cosmDir)
	selectedPackage := findPackageInRegistries(packageName, versionTag, cosmDir, registryNames)
	updateProjectWithDependency(project, packageName, versionTag, selectedPackage.RegistryName)
}

// Release updates the project version and publishes it to the remote repository and registries
func Release(cmd *cobra.Command, args []string) {
	project := loadProject("Project.json")
	ensureNoUncommittedChanges()
	ensureLocalRepoInSyncWithOrigin()
	newVersion := determineNewVersion(cmd, args, project.Version)
	validateNewVersion(newVersion, project.Version)
	ensureTagDoesNotExist(newVersion)
	registryName, _ := cmd.Flags().GetString("registry")
	registries := findHostingRegistries(project.Name, registryName)
	ensureRegistriesExist(registries, registryName)
	updateProjectVersion(project, newVersion)
	publishToGitRemote(newVersion)
	publishToRegistries(project, registries, newVersion)
	fmt.Printf("Released version '%s' for project '%s'\n", newVersion, project.Name)
}

// validateInitArgs checks the command-line arguments for validity
func validateInitArgs(args []string, cmd *cobra.Command) string {
	if len(args) == 0 {
		fmt.Println("Error: Package name is required")
		cmd.Usage()
		os.Exit(1)
	}
	packageName := args[0]
	if packageName == "" {
		fmt.Println("Error: Package name cannot be empty")
		os.Exit(1)
	}
	return packageName
}

// getInitFlags retrieves the language and version flags from the command
func getInitFlags(cmd *cobra.Command) (string, string) {
	language, _ := cmd.Flags().GetString("language")
	version, _ := cmd.Flags().GetString("version")
	if version == "" {
		version = "v0.1.0" // Default version
	}
	return language, version
}

// validateVersion ensures the version starts with 'v'
func validateVersion(version string) {
	if version[0] != 'v' {
		fmt.Printf("Error: Version '%s' must start with 'v'\n", version)
		os.Exit(1)
	}
}

// getGitAuthors retrieves the author info from git config or uses a default
func getGitAuthors() []string {
	name, errName := exec.Command("git", "config", "user.name").Output()
	email, errEmail := exec.Command("git", "config", "user.email").Output()
	if errName != nil || errEmail != nil || len(name) == 0 || len(email) == 0 {
		fmt.Println("Warning: Could not retrieve git user.name or user.email, defaulting to '[unknown]unknown@author.com'")
		return []string{"[unknown]unknown@author.com"}
	}
	gitName := strings.TrimSpace(string(name))
	gitEmail := strings.TrimSpace(string(email))
	return []string{fmt.Sprintf("[%s]%s", gitName, gitEmail)}
}

// ensureProjectFileDoesNotExist checks if Project.json already exists
func ensureProjectFileDoesNotExist(projectFile string) {
	if _, err := os.Stat(projectFile); !os.IsNotExist(err) {
		fmt.Printf("Error: Project.json already exists in this directory\n")
		os.Exit(1)
	}
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
func marshalProject(project types.Project) []byte {
	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling Project.json: %v\n", err)
		os.Exit(1)
	}
	return data
}

// writeProjectFile writes the project data to Project.json
func writeProjectFile(projectFile string, data []byte) {
	if err := os.WriteFile(projectFile, data, 0644); err != nil {
		fmt.Printf("Error writing Project.json: %v\n", err)
		os.Exit(1)
	}
}

// parseAddArgs validates and parses the package_name@version argument
func parseAddArgs(args []string) (string, string) {
	if len(args) != 1 {
		fmt.Println("Error: Exactly one argument required in the format <package_name>@v<version_number> (e.g., cosm add mypkg@v1.2.3)")
		os.Exit(1)
	}
	depArg := args[0]
	parts := strings.SplitN(depArg, "@", 2)
	if len(parts) != 2 {
		fmt.Printf("Error: Argument '%s' must be in the format <package_name>@v<version_number>\n", depArg)
		os.Exit(1)
	}
	packageName, versionTag := parts[0], parts[1]
	if packageName == "" {
		fmt.Println("Error: Package name cannot be empty")
		os.Exit(1)
	}
	if !strings.HasPrefix(versionTag, "v") {
		fmt.Printf("Error: Version '%s' must start with 'v'\n", versionTag)
		os.Exit(1)
	}
	return packageName, versionTag
}

// loadProject reads and parses the Project.json file
func loadProject(projectFile string) *types.Project {
	if _, err := os.Stat(projectFile); os.IsNotExist(err) {
		fmt.Printf("Error: No Project.json found in current directory\n")
		os.Exit(1)
	}
	data, err := os.ReadFile(projectFile)
	if err != nil {
		fmt.Printf("Error reading Project.json: %v\n", err)
		os.Exit(1)
	}
	var project types.Project
	if err := json.Unmarshal(data, &project); err != nil {
		fmt.Printf("Error parsing Project.json: %v\n", err)
		os.Exit(1)
	}
	if project.Deps == nil {
		project.Deps = make(map[string]string)
	}
	return &project
}

// getCosmDir retrieves the global .cosm directory
func getCosmDir() string {
	cosmDir, err := getGlobalCosmDir()
	if err != nil {
		fmt.Printf("Error getting global .cosm directory: %v\n", err)
		os.Exit(1)
	}
	return cosmDir
}

// loadRegistryNames loads the list of registry names from registries.json
func loadRegistryNames(cosmDir string) []string {
	registriesDir := filepath.Join(cosmDir, "registries")
	registriesFile := filepath.Join(registriesDir, "registries.json")
	if _, err := os.Stat(registriesFile); os.IsNotExist(err) {
		fmt.Println("Error: No registries found (run 'cosm registry init' first)")
		os.Exit(1)
	}
	data, err := os.ReadFile(registriesFile)
	if err != nil {
		fmt.Printf("Error reading registries.json: %v\n", err)
		os.Exit(1)
	}
	var registryNames []string
	if err := json.Unmarshal(data, &registryNames); err != nil {
		fmt.Printf("Error parsing registries.json: %v\n", err)
		os.Exit(1)
	}
	if len(registryNames) == 0 {
		fmt.Println("Error: No registries available to search for packages")
		os.Exit(1)
	}
	return registryNames
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
func selectPackageFromResults(packageName, versionTag string, foundPackages []packageLocation) packageLocation {
	if len(foundPackages) == 0 {
		fmt.Printf("Error: Package '%s' with version '%s' not found in any registry\n", packageName, versionTag)
		os.Exit(1)
	}
	if len(foundPackages) == 1 {
		return foundPackages[0]
	}
	return promptUserForRegistry(packageName, versionTag, foundPackages)
}

// findPackageInRegistries searches for a package across all registries
func findPackageInRegistries(packageName, versionTag, cosmDir string, registryNames []string) packageLocation {
	var foundPackages []packageLocation
	registriesDir := filepath.Join(cosmDir, "registries")

	for _, regName := range registryNames {
		if pkg, found := findPackageInRegistry(packageName, versionTag, registriesDir, regName); found {
			foundPackages = append(foundPackages, pkg)
		}
	}

	return selectPackageFromResults(packageName, versionTag, foundPackages)
}

// promptUserForRegistry handles multiple registry matches by prompting the user
func promptUserForRegistry(packageName, versionTag string, foundPackages []packageLocation) packageLocation {
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
		fmt.Printf("Error: Invalid selection '%s'. Must be a number between 1 and %d\n", choice, len(foundPackages))
		os.Exit(1)
	}
	return foundPackages[choiceNum-1]
}

// updateProjectWithDependency adds the dependency and saves the updated project
func updateProjectWithDependency(project *types.Project, packageName, versionTag string, registryName string) {
	if _, exists := project.Deps[packageName]; exists {
		fmt.Printf("Error: Dependency '%s' already exists in project\n", packageName)
		os.Exit(1)
	}
	project.Deps[packageName] = versionTag

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling Project.json: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("Project.json", data, 0644); err != nil {
		fmt.Printf("Error writing Project.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Added dependency '%s' %s from registry '%s' to project\n", packageName, versionTag, registryName)
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
func determineNewVersion(cmd *cobra.Command, args []string, currentVersion string) string {
	if len(args) == 1 {
		return args[0]
	}
	if len(args) > 1 {
		fmt.Println("Error: Too many arguments. Use 'cosm release v<version>' or a version flag (--patch, --minor, --major)")
		cmd.Usage()
		os.Exit(1)
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
		fmt.Println("Error: Only one of --patch, --minor, or --major can be specified")
		cmd.Usage()
		os.Exit(1)
	}
	if count == 0 {
		fmt.Println("Error: Specify a version (e.g., v1.2.3) or use --patch, --minor, or --major")
		cmd.Usage()
		os.Exit(1)
	}

	currentSemVer := parseSemVer(currentVersion)
	switch {
	case patch:
		return fmt.Sprintf("v%d.%d.%d", currentSemVer.Major, currentSemVer.Minor, currentSemVer.Patch+1)
	case minor:
		return fmt.Sprintf("v%d.%d.0", currentSemVer.Major, currentSemVer.Minor+1, 0)
	case major:
		return fmt.Sprintf("v%d.0.0", currentSemVer.Major+1, 0, 0)
	}
	return "" // Unreachable due to earlier checks
}

// semVer represents a semantic version (vX.Y.Z)
type semVer struct {
	Major, Minor, Patch int
}

// parseSemVer parses a version string into a semVer struct
func parseSemVer(version string) semVer {
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != 3 {
		fmt.Printf("Error: Invalid version format '%s'. Must be vX.Y.Z\n", version)
		os.Exit(1)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		fmt.Printf("Error: Invalid major version in '%s': %v\n", version, err)
		os.Exit(1)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		fmt.Printf("Error: Invalid minor version in '%s': %v\n", version, err)
		os.Exit(1)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		fmt.Printf("Error: Invalid patch version in '%s': %v\n", version, err)
		os.Exit(1)
	}
	return semVer{Major: major, Minor: minor, Patch: patch}
}

// validateNewVersion ensures the new version is valid and greater than the current
func validateNewVersion(newVersion, currentVersion string) {
	if !strings.HasPrefix(newVersion, "v") {
		fmt.Printf("Error: New version '%s' must start with 'v'\n", newVersion)
		os.Exit(1)
	}
	newSemVer := parseSemVer(newVersion)
	currentSemVer := parseSemVer(currentVersion)
	if newSemVer.Major < currentSemVer.Major ||
		(newSemVer.Major == currentSemVer.Major && newSemVer.Minor < currentSemVer.Minor) ||
		(newSemVer.Major == currentSemVer.Major && newSemVer.Minor == currentSemVer.Minor && newSemVer.Patch <= currentSemVer.Patch) {
		fmt.Printf("Error: New version '%s' must be greater than current version '%s'\n", newVersion, currentVersion)
		os.Exit(1)
	}
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
	Dir        string
	MetaFile   string
	PackageDir string
}

// findHostingRegistries identifies registries hosting the package
func findHostingRegistries(packageName, specificRegistry string) []registryInfo {
	cosmDir := getCosmDir()
	registryNames := loadRegistryNames(cosmDir)
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
				Dir:        filepath.Join(registriesDir, regName),
				MetaFile:   filepath.Join(registriesDir, regName, "registry.json"),
				PackageDir: packageDir,
			})
		}
	}
	return registries
}

// updateProjectVersion updates the version in Project.json and saves it
func updateProjectVersion(project *types.Project, newVersion string) {
	project.Version = newVersion
	data := marshalProject(*project)
	writeProjectFile("Project.json", data)
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
func publishToRegistries(project *types.Project, registries []registryInfo, newVersion string) {
	for _, reg := range registries {
		pullRegistryUpdates(reg.Dir, reg.Name)
		updateRegistryVersions(reg.PackageDir, newVersion, project, reg.Name)
		commitAndPushRegistryChanges(reg.Dir, reg.Name, project.Name, newVersion)
	}
}

// updateRegistryVersions updates versions.json and adds a specs file for the new version
func updateRegistryVersions(packageDir, newVersion string, project *types.Project, registryName string) {
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
	sha1Output, err := exec.Command("git", "rev-list", "-n", "1", newVersion).Output()
	if err != nil {
		fmt.Printf("Error getting SHA1 for tag '%s' in registry '%s': %v\n", newVersion, registryName, err)
		os.Exit(1)
	}
	sha1 := strings.TrimSpace(string(sha1Output))
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

func Rm(cmd *cobra.Command, args []string) {
}

func Develop(cmd *cobra.Command, args []string) {

}

func Free(cmd *cobra.Command, args []string) {

}

func Upgrade(cmd *cobra.Command, args []string) {

}

func Downgrade(cmd *cobra.Command, args []string) {

}
