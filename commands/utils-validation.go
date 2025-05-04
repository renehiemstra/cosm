package commands

import (
	"cosm/types"
	"fmt"

	"github.com/google/uuid"
)

// validateProject validates a project struct for registry operations
func validateProject(project types.Project) error {
	if project.Name == "" {
		return fmt.Errorf("Project.json  does not contain a valid package name")
	}
	if project.UUID == "" {
		return fmt.Errorf("Project.json does not contain a valid UUID")
	}
	if _, err := uuid.Parse(project.UUID); err != nil {
		return fmt.Errorf("invalid UUID '%s' in Project.json: %v", project.UUID, err)
	}
	if project.Version == "" {
		return fmt.Errorf("Project.json does not contain a version")
	}
	// Validate version parsing
	_, err := ParseSemVer(project.Version)
	if err != nil {
		return fmt.Errorf("invalid version in Project.json: %v", err)
	}
	return nil
}
