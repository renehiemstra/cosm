package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PrintVersion prints the version of the cosm tool and exits
func PrintVersion() {
	fmt.Printf("cosm version %s\n", Version)
	os.Exit(0)
}

// validateVersion ensures the version starts with 'v'
func validateVersion(version string) error {
	if len(version) == 0 || version[0] != 'v' {
		return fmt.Errorf("version '%s' must start with 'v'", version)
	}
	return nil
}

// ParseSemVer parses a semantic version string into its components
func ParseSemVer(version string) (semVer, error) {
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

// semVer represents a semantic version (vX.Y.Z)
type semVer struct {
	Major, Minor, Patch int
}

// MaxSemVer returns the higher of two semantic versions
func MaxSemVer(v1, v2 string) (string, error) {
	s1, err := ParseSemVer(v1)
	if err != nil {
		return "", err
	}
	s2, err := ParseSemVer(v2)
	if err != nil {
		return "", err
	}
	if s1.Major > s2.Major {
		return v1, nil
	}
	if s1.Major < s2.Major {
		return v2, nil
	}
	if s1.Minor > s2.Minor {
		return v1, nil
	}
	if s1.Minor < s2.Minor {
		return v2, nil
	}
	if s1.Patch >= s2.Patch {
		return v1, nil
	}
	return v2, nil
}

// GetMajorVersion extracts the major version number as a string (e.g., "v1" from "v1.2.0")
func GetMajorVersion(version string) (string, error) {
	s, err := ParseSemVer(version)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("v%d", s.Major), nil
}

// validateNewVersion ensures the new version is valid and allowed
func validateNewVersion(newVersion, currentVersion string) error {
	// Parse versions
	currVer, err := ParseSemVer(currentVersion)
	if err != nil {
		return fmt.Errorf("invalid current version %q: %v", currentVersion, err)
	}
	newVer, err := ParseSemVer(newVersion)
	if err != nil {
		return fmt.Errorf("invalid new version %q: %v", newVersion, err)
	}

	// Allow same version if not tagged, otherwise require newer
	if newVersion == currentVersion {
		return nil // Tag existence checked later by ensureTagDoesNotExist
	}

	// Compare versions: newVer must be greater than currVer
	if newVer.Major < currVer.Major {
		return fmt.Errorf("new version %q must be greater than current version %q", newVersion, currentVersion)
	}
	if newVer.Major == currVer.Major {
		if newVer.Minor < currVer.Minor {
			return fmt.Errorf("new version %q must be greater than current version %q", newVersion, currentVersion)
		}
		if newVer.Minor == currVer.Minor && newVer.Patch <= currVer.Patch {
			return fmt.Errorf("new version %q must be greater than current version %q", newVersion, currentVersion)
		}
	}
	return nil
}
