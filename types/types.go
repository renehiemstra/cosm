package types

// PackageInfo represents metadata for a package in a registry
type PackageInfo struct {
	UUID   string `json:"uuid"`
	GitURL string `json:"giturl"`
}

// Registry represents a package registry
type Registry struct {
	Name     string                 `json:"name"`
	UUID     string                 `json:"uuid"`
	GitURL   string                 `json:"giturl"`
	Packages map[string]PackageInfo `json:"packages"`
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

// Specs represents the metadata for a package version
type Specs struct {
	Name    string            `json:"name"`
	UUID    string            `json:"uuid"`
	Version string            `json:"version"`
	GitURL  string            `json:"giturl"`
	SHA1    string            `json:"sha1"`
	Deps    map[string]string `json:"deps"`
}

// BuildList represents the minimum version dependencies for a package version
type BuildList struct {
	Dependencies map[string]BuildListDependency `json:"dependencies"`
}

// BuildListDependency represents a single dependency in the build list
type BuildListDependency struct {
	Name    string `json:"name"`
	UUID    string `json:"uuid"`
	Version string `json:"version"`
	GitURL  string `json:"giturl"`
	SHA1    string `json:"sha1"`
}
