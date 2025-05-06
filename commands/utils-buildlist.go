package commands

import (
	"cosm/types"
	"fmt"
	"strings"
)

// generateBuildList creates a build list using Minimum Version Selection (MVS),
// including direct dependencies from project.Deps and transitive dependencies
// from dependency build lists, taking the maximum version for shared dependencies.
func generateBuildList(project *types.Project, registriesDir string) (types.BuildList, error) {
	buildList := types.BuildList{Dependencies: make(map[string]types.BuildListDependency)}

	// Process direct dependencies
	for key, dep := range project.Deps {
		depUUID, err := extractUUIDFromKey(key)
		if err != nil {
			return types.BuildList{}, err
		}
		specs, depBuildList, err := findDependency(dep.Name, dep.Version, depUUID, registriesDir)
		if err != nil {
			return types.BuildList{}, err
		}
		key, entry, err := createDependencyEntry(dep.Name, dep.Version, depUUID, specs)
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

// extractUUIDFromKey extracts the UUID from a dependency key formatted as <uuid>@<major version>
func extractUUIDFromKey(key string) (string, error) {
	parts := strings.Split(key, "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid dependency key format: %s", key)
	}
	if parts[0] == "" {
		return "", fmt.Errorf("UUID is empty in key: %s", key)
	}
	return parts[0], nil
}

// findDependency searches all registries for a dependency with matching name, UUID, and version
func findDependency(depName, depVersion, depUUID, registriesDir string) (types.Specs, types.BuildList, error) {
	registryNames, err := loadRegistryNames(registriesDir)
	if err != nil {
		return types.Specs{}, types.BuildList{}, fmt.Errorf("failed to load registry names: %v", err)
	}

	for _, regName := range registryNames {
		reg, _, err := LoadRegistryMetadata(registriesDir, regName)
		if err != nil {
			continue
		}
		if pkgInfo, exists := reg.Packages[depName]; exists && pkgInfo.UUID == depUUID {
			specs, err := loadSpecs(registriesDir, regName, depName, depVersion)
			if err != nil {
				continue
			}
			if specs.Version != depVersion {
				continue
			}
			buildList, err := loadBuildList(registriesDir, regName, depName, depVersion)
			if err != nil {
				return types.Specs{}, types.BuildList{}, fmt.Errorf("failed to load build list for '%s@%s' in registry '%s': %v", depName, depVersion, regName, err)
			}
			return specs, buildList, nil
		}
	}
	return types.Specs{}, types.BuildList{}, fmt.Errorf("dependency '%s@%s' with UUID '%s' not found in any registry", depName, depVersion, depUUID)
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
