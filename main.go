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

// ValidRegistries is a list of allowed registry names
var ValidRegistries = []string{"cosmic-hub", "local"}

// Registry represents a package registry
type Registry struct {
	Name        string    `json:"name"`
	GitURL      string    `json:"giturl"`
	LastUpdated time.Time `json:"last_updated,omitempty"`
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

	// cosm activate
	// An interactive environment is loaded, which initialized all environment variables
	// needed for dependency management. The interactive prompt looks like
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

	// cosm registry status <registry name>
	// Gives an overview of the packages registered to the registry. Can be evaluated anywhere.
	var registryStatusCmd = &cobra.Command{
		Use:   "status [registry-name]",
		Short: "Show contents of a registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			// Validate registry name
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

	// cosm registry init <registry name> <giturl>
	// Adds a new package registry with name name (in .cosm/registries) with remote located
	// at giturl. The giturl should point to an empty remote git repository.
	var registryInitCmd = &cobra.Command{
		Use:   "init [registry-name] [giturl]",
		Short: "Initialize a new registry",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			gitURL := args[1]

			// Create .cosm directory if it doesn’t exist
			cosmDir := ".cosm"
			if err := os.MkdirAll(cosmDir, 0755); err != nil {
				fmt.Printf("Error creating .cosm directory: %v\n", err)
				os.Exit(1)
			}

			// Path to registries.json
			registriesFile := filepath.Join(cosmDir, "registries.json")

			// Load existing registries (or initialize empty list)
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

			// Check for duplicate registry name
			for _, reg := range registries {
				if reg.Name == registryName {
					fmt.Printf("Error: Registry '%s' already exists\n", registryName)
					os.Exit(1)
				}
			}

			// Add new registry
			registries = append(registries, Registry{Name: registryName, GitURL: gitURL})

			// Write updated registries back to file
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

	// "registry clone <giturl>"
	// Adds an existing package registry (in .cosm/registries) with remote located
	// at giturl. The giturl should point to a valid existing package registry.
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

			// Generate a name from the giturl (e.g., last part of the URL)
			name := filepath.Base(gitURL)
			if name == "" || name == "." {
				fmt.Printf("Error: Could not derive a valid registry name from %s\n", gitURL)
				os.Exit(1)
			}

			// Load existing registries
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

			// Check for duplicate
			for _, reg := range registries {
				if reg.Name == name {
					fmt.Printf("Error: Registry '%s' already exists\n", name)
					os.Exit(1)
				}
			}

			// Add the registry
			registries = append(registries, Registry{Name: name, GitURL: gitURL})
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

	// "registry delete <registry name> [--force]"
	var registryDeleteCmd = &cobra.Command{
		Use:   "delete [registry-name]",
		Short: "Delete a registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			force, _ := cmd.Flags().GetBool("force")
			cosmDir := ".cosm"
			registriesFile := filepath.Join(cosmDir, "registries.json")

			// Load existing registries
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

			// Find and remove the registry
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

			// Write updated registries back
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

	// "registry update <registry name>" and "registry update --all"
	var registryUpdateCmd = &cobra.Command{
		Use:   "update [registry-name]",
		Short: "Update and synchronize a registry with its remote",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			cosmDir := ".cosm"
			registriesFile := filepath.Join(cosmDir, "registries.json")

			// Load existing registries
			var registries []Registry
			if data, err := os.ReadFile(registriesFile); err == nil {
				if err := json.Unmarshal(data, &registries); err != nil { // Fixed typo: ®istries -> &registries
					fmt.Printf("Error parsing registries.json: %v\n", err)
					os.Exit(1)
				}
			} else if os.IsNotExist(err) {
				fmt.Printf("Error: No registries found to update '%s' from\n", registryName)
				os.Exit(1)
			} else {
				fmt.Printf("Error reading registries.json: %v\n", err)
				os.Exit(1)
			}

			// Find and update the registry
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

			// Write updated registries back
			data, err := json.MarshalIndent(registries, "", "  ")
			if err != nil {
				fmt.Printf("Error marshaling registries: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(registriesFile, data, 0644); err != nil {
				fmt.Printf("Error writing registries.json: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Updated registry '%s'\n", registryName)
		},
	}

	// "registry add <registry name> v<version tag> <giturl>"
	var registryAddCmd = &cobra.Command{
		Use:   "add [registry-name] v<version> [giturl]",
		Short: "Add a package to a registry with a version",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			versionTag := args[1]
			gitURL := args[2]
			if versionTag[0] != 'v' {
				fmt.Println("Error: version must start with 'v' (e.g., v1.0.0)")
				os.Exit(1)
			}
			fmt.Printf("Added package to registry '%s' with version %s from %s\n", registryName, versionTag, gitURL)
		},
	}

	// "registry rm <registry name> <package name> [--force]" and "rm <registry name> <package name> v<version> [--force]"
	var registryRmCmd = &cobra.Command{
		Use:   "rm [registry-name] [package-name] [v<version>]",
		Short: "Remove a package from a registry",
		Args:  cobra.RangeArgs(2, 3),
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			packageName := args[1]
			force, _ := cmd.Flags().GetBool("force")
			if len(args) == 3 {
				version := args[2]
				if version[0] != 'v' {
					fmt.Println("Error: version must start with 'v' (e.g., v1.0.0)")
					os.Exit(1)
				}
				if force {
					fmt.Printf("Force removed package '%s' version %s from registry '%s'\n", packageName, version, registryName)
				} else {
					fmt.Printf("Removed package '%s' version %s from registry '%s'\n", packageName, version, registryName)
				}
			} else {
				if force {
					fmt.Printf("Force removed package '%s' from registry '%s'\n", packageName, registryName)
				} else {
					fmt.Printf("Removed package '%s' from registry '%s'\n", packageName, registryName)
				}
			}
		},
	}
	registryRmCmd.Flags().BoolP("force", "f", false, "Force removal of the package")

	// Add subcommands to registry
	registryCmd.AddCommand(registryStatusCmd)
	registryCmd.AddCommand(registryInitCmd)
	registryCmd.AddCommand(registryCloneCmd)
	registryCmd.AddCommand(registryDeleteCmd)
	registryCmd.AddCommand(registryUpdateCmd)
	registryCmd.AddCommand(registryAddCmd)
	registryCmd.AddCommand(registryRmCmd)

	// Add commands to root
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(activateCmd)
	rootCmd.AddCommand(registryCmd)

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
