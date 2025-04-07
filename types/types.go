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

type Project struct {
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
}
