// cosm --version
// cosm status
// cosm activate

// cosm registry status <registry name>
// cosm registry init <registry name> <giturl>
// cosm registry clone <giturl>
// cosm registry delete <registry name> [--force]
// cosm registry update <registry name>
// cosm registry update --all
// cosm registry add <registry name> v<version tag> <giturl>
// cosm registry rm <registry name> <package name> [--force]
// cosm registry rm <registry name> <package name> v<version> [--force]

// cosm init <package name>
// cosm init <package name> --language <language>
// cosm init <package name> --template <language/template>
// cosm add <name> v<version>
// cosm rm <name>

// cosm release v<version>
// cosm release --patch
// cosm release --minor
// cosm release --major

// cosm develop <package name>
// cosm free <package name>

// cosm upgrade <name>
// cosm upgrade <name> v<x>
// cosm upgrade <name> v<x.y>
// cosm upgrade <name> v<x.y.z>
// cosm upgrade <name> v<x.y.z-alpha>
// cosm upgrade <name> v<x.y>
// cosm upgrade <name> v<x.y.z>
// cosm upgrade <name> --latest
// cosm upgrade --all
// cosm upgrade --all --latest

// cosm downgrade <name> v<version>

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

var ValidRegistries = []string{"cosmic-hub", "local"}

type Registry struct {
	Name        string              `json:"name"`
	GitURL      string              `json:"giturl"`
	Packages    map[string][]string `json:"packages,omitempty"` // Map of package names to version tags
	LastUpdated time.Time           `json:"last_updated,omitempty"`
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "cosm",
		Short: "A cosmic package manager",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Welcome to Cosm! Use a subcommand like 'status', 'activate', or 'registry'.")
		},
	}

	var versionFlag bool
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print the version number")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if versionFlag {
			fmt.Printf("cosm version %s\n", version)
			os.Exit(0)
		}
	}

	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show the current cosmic status",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Cosmic Status:")
			fmt.Println("  Orbit: Stable")
			fmt.Println("  Systems: All green")
			fmt.Println("  Pending updates: None")
			fmt.Println("Run 'cosm status' in a project directory for more details.")
		},
	}

	var activateCmd = &cobra.Command{
		Use:   "activate",
		Short: "Activate the current project",
		Run: func(cmd *cobra.Command, args []string) {
			if _, err := os.Stat("cosm.json"); os.IsNotExist(err) {
				fmt.Println("Error: No project found in current directory (missing cosm.json)")
				os.Exit(1)
			} else {
				fmt.Println("Activated current project")
			}
		},
	}

	var registryCmd = &cobra.Command{
		Use:   "registry",
		Short: "Manage package registries",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Registry command requires a subcommand (e.g., 'status', 'init').")
		},
	}

	var registryStatusCmd = &cobra.Command{
		Use:   "status [registry-name]",
		Short: "Show contents of a registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			valid := false
			for _, validReg := range ValidRegistries {
				if registryName == validReg {
					valid = true
					break
				}
			}
			if !valid {
				fmt.Printf("Error: '%s' is not a valid registry name. Valid options: %v\n", registryName, ValidRegistries)
				os.Exit(1)
			}
			fmt.Printf("Status for registry '%s':\n", registryName)
			fmt.Println("  Available packages:")
			fmt.Printf("    - %s-pkg1 (v1.0.0)\n", registryName)
			fmt.Printf("    - %s-pkg2 (v2.1.3)\n", registryName)
			fmt.Println("  Last updated: 2025-04-05")
		},
	}

	var registryInitCmd = &cobra.Command{
		Use:   "init [registry-name] [giturl]",
		Short: "Initialize a new registry",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			gitURL := args[1]
			cosmDir := ".cosm"
			if err := os.MkdirAll(cosmDir, 0755); err != nil {
				fmt.Printf("Error creating .cosm directory: %v\n", err)
				os.Exit(1)
			}
			registriesFile := filepath.Join(cosmDir, "registries.json")
			var registries []Registry
			if data, err := os.ReadFile(registriesFile); err == nil {
				if err := json.Unmarshal(data, &registries); err != nil {
					fmt.Printf("Error parsing registries.json: %v\n", err)
					os.Exit(1)
				}
			} else if !os.IsNotExist(err) {
				fmt.Printf("Error reading registries.json: %v\n", err)
				os.Exit(1)
			}
			for _, reg := range registries {
				if reg.Name == registryName {
					fmt.Printf("Error: Registry '%s' already exists\n", registryName)
					os.Exit(1)
				}
			}
			registries = append(registries, Registry{
				Name:     registryName,
				GitURL:   gitURL,
				Packages: make(map[string][]string), // Initialize empty package map
			})
			data, err := json.MarshalIndent(registries, "", "  ")
			if err != nil {
				fmt.Printf("Error marshaling registries: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(registriesFile, data, 0644); err != nil {
				fmt.Printf("Error writing registries.json: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Initialized registry '%s' with Git URL: %s\n", registryName, gitURL)
		},
	}

	var registryCloneCmd = &cobra.Command{
		Use:   "clone [giturl]",
		Short: "Clone a registry from a Git URL",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			gitURL := args[0]
			cosmDir := ".cosm"
			if err := os.MkdirAll(cosmDir, 0755); err != nil {
				fmt.Printf("Error creating .cosm directory: %v\n", err)
				os.Exit(1)
			}
			name := filepath.Base(gitURL)
			if name == "" || name == "." {
				fmt.Printf("Error: Could not derive a valid registry name from %s\n", gitURL)
				os.Exit(1)
			}
			registriesFile := filepath.Join(cosmDir, "registries.json")
			var registries []Registry
			if data, err := os.ReadFile(registriesFile); err == nil {
				if err := json.Unmarshal(data, &registries); err != nil {
					fmt.Printf("Error parsing registries.json: %v\n", err)
					os.Exit(1)
				}
			} else if !os.IsNotExist(err) {
				fmt.Printf("Error reading registries.json: %v\n", err)
				os.Exit(1)
			}
			for _, reg := range registries {
				if reg.Name == name {
					fmt.Printf("Error: Registry '%s' already exists\n", name)
					os.Exit(1)
				}
			}
			registries = append(registries, Registry{
				Name:     name,
				GitURL:   gitURL,
				Packages: make(map[string][]string), // Initialize empty package map
			})
			data, err := json.MarshalIndent(registries, "", "  ")
			if err != nil {
				fmt.Printf("Error marshaling registries: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(registriesFile, data, 0644); err != nil {
				fmt.Printf("Error writing registries.json: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Cloned registry '%s' from %s\n", name, gitURL)
		},
	}

	var registryDeleteCmd = &cobra.Command{
		Use:   "delete [registry-name]",
		Short: "Delete a registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			force, _ := cmd.Flags().GetBool("force")
			cosmDir := ".cosm"
			registriesFile := filepath.Join(cosmDir, "registries.json")
			var registries []Registry
			if data, err := os.ReadFile(registriesFile); err == nil {
				if err := json.Unmarshal(data, &registries); err != nil {
					fmt.Printf("Error parsing registries.json: %v\n", err)
					os.Exit(1)
				}
			} else if os.IsNotExist(err) {
				fmt.Printf("Error: No registries found to delete '%s' from\n", registryName)
				os.Exit(1)
			} else {
				fmt.Printf("Error reading registries.json: %v\n", err)
				os.Exit(1)
			}
			found := false
			for i, reg := range registries {
				if reg.Name == registryName {
					registries = append(registries[:i], registries[i+1:]...)
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("Error: Registry '%s' not found\n", registryName)
				os.Exit(1)
			}
			data, err := json.MarshalIndent(registries, "", "  ")
			if err != nil {
				fmt.Printf("Error marshaling registries: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(registriesFile, data, 0644); err != nil {
				fmt.Printf("Error writing registries.json: %v\n", err)
				os.Exit(1)
			}
			if force {
				fmt.Printf("Force deleted registry '%s'\n", registryName)
			} else {
				fmt.Printf("Deleted registry '%s'\n", registryName)
			}
		},
	}
	registryDeleteCmd.Flags().BoolP("force", "f", false, "Force deletion of the registry")

	var registryUpdateCmd = &cobra.Command{
		Use:   "update [registry-name | --all]",
		Short: "Update and synchronize a registry with its remote",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			all, _ := cmd.Flags().GetBool("all")
			cosmDir := ".cosm"
			registriesFile := filepath.Join(cosmDir, "registries.json")

			var registries []Registry
			if data, err := os.ReadFile(registriesFile); err == nil {
				if err := json.Unmarshal(data, &registries); err != nil {
					fmt.Printf("Error parsing registries.json: %v\n", err)
					os.Exit(1)
				}
			} else if os.IsNotExist(err) {
				fmt.Printf("Error: No registries found to update\n")
				os.Exit(1)
			} else {
				fmt.Printf("Error reading registries.json: %v\n", err)
				os.Exit(1)
			}

			if all {
				if len(registries) == 0 {
					fmt.Println("No registries to update")
					return
				}
				for i := range registries {
					registries[i].LastUpdated = time.Now()
				}
				fmt.Println("Updated all registries")
			} else if len(args) > 0 {
				registryName := args[0]
				found := false
				for i, reg := range registries {
					if reg.Name == registryName {
						registries[i].LastUpdated = time.Now()
						found = true
						break
					}
				}
				if !found {
					fmt.Printf("Error: Registry '%s' not found\n", registryName)
					os.Exit(1)
				}
				fmt.Printf("Updated registry '%s'\n", registryName)
			} else {
				fmt.Println("Error: 'update' requires a registry name or --all")
				os.Exit(1)
			}

			data, err := json.MarshalIndent(registries, "", "  ")
			if err != nil {
				fmt.Printf("Error marshaling registries: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(registriesFile, data, 0644); err != nil {
				fmt.Printf("Error writing registries.json: %v\n", err)
				os.Exit(1)
			}
		},
	}
	registryUpdateCmd.Flags().Bool("all", false, "Update all registries")

	var registryAddCmd = &cobra.Command{
		Use:   "add [registry-name] v<version> [giturl]",
		Short: "Register a package version to a registry",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			versionTag := args[1]
			gitURL := args[2]

			if versionTag[0] != 'v' {
				fmt.Printf("Error: Version '%s' must start with 'v'\n", versionTag)
				os.Exit(1)
			}

			cosmDir := ".cosm"
			if err := os.MkdirAll(cosmDir, 0755); err != nil {
				fmt.Printf("Error creating .cosm directory: %v\n", err)
				os.Exit(1)
			}
			registriesFile := filepath.Join(cosmDir, "registries.json")

			var registries []Registry
			if data, err := os.ReadFile(registriesFile); err == nil {
				if err := json.Unmarshal(data, &registries); err != nil {
					fmt.Printf("Error parsing registries.json: %v\n", err)
					os.Exit(1)
				}
			} else if !os.IsNotExist(err) {
				fmt.Printf("Error reading registries.json: %v\n", err)
				os.Exit(1)
			}

			found := false
			for i := range registries {
				if registries[i].Name == registryName {
					if registries[i].Packages == nil {
						registries[i].Packages = make(map[string][]string)
					}
					pkgName := "pkg" // Placeholder
					for _, v := range registries[i].Packages[pkgName] {
						if v == versionTag {
							fmt.Printf("Error: Version '%s' already exists in registry '%s' for package '%s'\n", versionTag, registryName, pkgName)
							os.Exit(1)
						}
					}
					registries[i].Packages[pkgName] = append(registries[i].Packages[pkgName], versionTag)
					registries[i].GitURL = gitURL
					registries[i].LastUpdated = time.Now()
					found = true
					break
				}
			}
			if !found {
				registries = append(registries, Registry{
					Name:        registryName,
					GitURL:      gitURL,
					Packages:    map[string][]string{"pkg": {versionTag}},
					LastUpdated: time.Now(),
				})
			}

			data, err := json.MarshalIndent(registries, "", "  ")
			if err != nil {
				fmt.Printf("Error marshaling registries: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(registriesFile, data, 0644); err != nil {
				fmt.Printf("Error writing registries.json: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Added version '%s' to registry '%s' from %s\n", versionTag, registryName, gitURL)
		},
	}

	var registryRmCmd = &cobra.Command{
		Use:   "rm [registry-name] [package-name] [v<version>]",
		Short: "Remove a package or version from a registry",
		Args:  cobra.RangeArgs(2, 3),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			packageName := args[1]
			force, _ := cmd.Flags().GetBool("force")
			cosmDir := ".cosm"
			registriesFile := filepath.Join(cosmDir, "registries.json")

			var registries []Registry
			if data, err := os.ReadFile(registriesFile); err == nil {
				if err := json.Unmarshal(data, &registries); err != nil {
					fmt.Printf("Error parsing registries.json: %v\n", err)
					os.Exit(1)
				}
			} else if os.IsNotExist(err) {
				fmt.Printf("Error: No registries found to remove from\n")
				os.Exit(1)
			} else {
				fmt.Printf("Error reading registries.json: %v\n", err)
				os.Exit(1)
			}

			foundReg := false
			regIndex := -1
			for i, reg := range registries {
				if reg.Name == registryName {
					foundReg = true
					regIndex = i
					break
				}
			}
			if !foundReg {
				fmt.Printf("Error: Registry '%s' not found\n", registryName)
				os.Exit(1)
			}

			if registries[regIndex].Packages == nil {
				registries[regIndex].Packages = make(map[string][]string)
			}

			if len(args) == 3 { // Remove specific version
				versionTag := args[2]
				if versionTag[0] != 'v' {
					fmt.Printf("Error: Version '%s' must start with 'v'\n", versionTag)
					os.Exit(1)
				}
				versions, exists := registries[regIndex].Packages[packageName]
				if !exists || len(versions) == 0 {
					fmt.Printf("Error: Package '%s' not found in registry '%s'\n", packageName, registryName)
					os.Exit(1)
				}
				foundVer := false
				for j, v := range versions {
					if v == versionTag {
						registries[regIndex].Packages[packageName] = append(versions[:j], versions[j+1:]...)
						foundVer = true
						break
					}
				}
				if !foundVer {
					fmt.Printf("Error: Version '%s' not found for package '%s' in registry '%s'\n", versionTag, packageName, registryName)
					os.Exit(1)
				}
				if len(registries[regIndex].Packages[packageName]) == 0 {
					delete(registries[regIndex].Packages, packageName)
				}
				registries[regIndex].LastUpdated = time.Now()
				if force {
					fmt.Printf("Force removed version '%s' from package '%s' in registry '%s'\n", versionTag, packageName, registryName)
				} else {
					fmt.Printf("Removed version '%s' from package '%s' in registry '%s'\n", versionTag, packageName, registryName)
				}
			} else { // Remove entire package
				if _, exists := registries[regIndex].Packages[packageName]; !exists {
					fmt.Printf("Error: Package '%s' not found in registry '%s'\n", packageName, registryName)
					os.Exit(1)
				}
				delete(registries[regIndex].Packages, packageName)
				registries[regIndex].LastUpdated = time.Now()
				if force {
					fmt.Printf("Force removed package '%s' from registry '%s'\n", packageName, registryName)
				} else {
					fmt.Printf("Removed package '%s' from registry '%s'\n", packageName, registryName)
				}
			}

			data, err := json.MarshalIndent(registries, "", "  ")
			if err != nil {
				fmt.Printf("Error marshaling registries: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(registriesFile, data, 0644); err != nil {
				fmt.Printf("Error writing registries.json: %v\n", err)
				os.Exit(1)
			}
		},
	}
	registryRmCmd.Flags().BoolP("force", "f", false, "Force removal of the package or version")

	registryCmd.AddCommand(registryStatusCmd)
	registryCmd.AddCommand(registryInitCmd)
	registryCmd.AddCommand(registryCloneCmd)
	registryCmd.AddCommand(registryDeleteCmd)
	registryCmd.AddCommand(registryUpdateCmd)
	registryCmd.AddCommand(registryAddCmd)
	registryCmd.AddCommand(registryRmCmd)

	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(activateCmd)
	rootCmd.AddCommand(registryCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
