package types

import "time"

type Registry struct {
	Name        string              `json:"name"`
	GitURL      string              `json:"giturl"`
	Packages    map[string][]string `json:"packages,omitempty"`
	LastUpdated time.Time           `json:"last_updated,omitempty"`
}

type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Develop bool   `json:"develop,omitempty"` // Indicates development mode
}

// Project represents a project configuration
type Project struct {
	Name     string            `json:"name"`
	UUID     string            `json:"uuid"`
	Authors  []string          `json:"authors"`
	Language string            `json:"language,omitempty"`
	Version  string            `json:"version"`
	Deps     map[string]string `json:"deps,omitempty"` // Changed from []Dependency to map[string]string
}
