package commands

import (
	"cosm/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// generateBuildList creates a build list using Minimum Version Selection (MVS),
// including direct dependencies from project.Deps and transitive dependencies
// from dependency build lists, taking the maximum version for shared dependencies.
func generateBuildList(project *types.Project, registriesDir string) (types.BuildList, error) {
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
		if pkginfo, exists := reg.Packages[depName]; exists {
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
			return pkginfo.UUID, specs, buildList, nil
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
