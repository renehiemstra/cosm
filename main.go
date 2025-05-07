// cosm --version
// cosm status
// cosm activate

// cosm registry status <registry name>
// cosm registry init <registry name> <giturl>
// cosm registry clone <giturl>
// cosm registry delete <registry name> [--force]
// cosm registry update <registry name>
// cosm registry update --all
// cosm registry add <registry name> <giturl>
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
	"cosm/commands"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version string // Populated by -ldflags during build

// PrintVersion prints the version of the cosm tool and exits
func PrintVersion() {
	fmt.Printf("cosm version %s\n", version)
	os.Exit(0)
}

func main() {

	// Initialize COSM_DEPOT_PATH
	if err := commands.InitializeCosm(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize COSM_DEPOT_PATH: %v\n", err)
		os.Exit(1)
	}

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
			PrintVersion()
		}
	}

	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show the current cosmic status",
		Run:   commands.Status, // Call from commands package,
	}

	var activateCmd = &cobra.Command{
		Use:          "activate",
		Short:        "Activate the current project",
		RunE:         commands.Activate,
		SilenceUsage: true, // Prevent usage output in stderr
	}

	// initCmd initializes a new project
	var initCmd = &cobra.Command{
		Use:          "init <package-name> [version]",
		Short:        "Initialize a new project",
		Args:         cobra.RangeArgs(1, 2),
		RunE:         commands.Init,
		SilenceUsage: true,
	}
	initCmd.Flags().StringP("version", "v", "", "Version of the project (default: v0.1.0)")
	initCmd.Flags().StringP("language", "l", "", "Language of the project (not allowed with --template)")
	initCmd.Flags().StringP("template", "t", "", "Path to template directory (relative to .cosm/templates/, e.g., go/mytemplate)")

	var addCmd = &cobra.Command{
		Use:          "add <package_name> [v<version>]",
		Short:        "Add a dependency to the project",
		Args:         cobra.RangeArgs(1, 2),
		RunE:         commands.Add,
		SilenceUsage: true,
	}

	var rmCmd = &cobra.Command{
		Use:          "rm [name]",
		Short:        "Remove a dependency from the project",
		Args:         cobra.ExactArgs(1),
		RunE:         commands.Rm,
		SilenceUsage: true, // Prevent usage output in stderr
	}

	var releaseCmd = &cobra.Command{
		Use:          "release [v<version>]",
		Short:        "Update the project version and publish a release",
		Args:         cobra.MaximumNArgs(1),
		RunE:         commands.Release,
		SilenceUsage: true, // Prevent usage output in stderr
	}
	releaseCmd.Flags().Bool("patch", false, "Increment the patch version")
	releaseCmd.Flags().Bool("minor", false, "Increment the minor version")
	releaseCmd.Flags().Bool("major", false, "Increment the major version")
	releaseCmd.Flags().String("registry", "", "Specify a registry to release to")

	var developCmd = &cobra.Command{
		Use:   "develop [package-name]",
		Short: "Switch an existing dependency to development mode",
		Args:  cobra.ExactArgs(1),
		Run:   commands.Develop,
	}

	var freeCmd = &cobra.Command{
		Use:   "free [package-name]",
		Short: "Close development mode for an existing dependency",
		Args:  cobra.ExactArgs(1),
		Run:   commands.Free,
	}

	var upgradeCmd = &cobra.Command{
		Use:   "upgrade [name] [v<version>]",
		Short: "Upgrade a dependency or all dependencies",
		Args:  cobra.RangeArgs(0, 2),
		Run:   commands.Upgrade,
	}
	upgradeCmd.Flags().Bool("all", false, "Upgrade all direct dependencies")
	upgradeCmd.Flags().Bool("latest", false, "Use the latest version instead of the latest compatible version")

	var downgradeCmd = &cobra.Command{
		Use:   "downgrade [name] v<version>",
		Short: "Downgrade a dependency to an older version",
		Args:  cobra.ExactArgs(2),
		Run:   commands.Downgrade,
	}

	var registryCmd = &cobra.Command{
		Use:   "registry",
		Short: "Manage package registries",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Registry command requires a subcommand (e.g., 'status', 'init').")
		},
	}

	var registryStatusCmd = &cobra.Command{
		Use:          "status [registry-name]",
		Short:        "Print an overview of packages in a registry",
		Args:         cobra.ExactArgs(1),
		RunE:         commands.RegistryStatus, // Changed from Run to RunE
		SilenceUsage: true,                    // Prevent usage output in stderr
	}

	var registryInitCmd = &cobra.Command{
		Use:          "init [registry-name] [giturl]",
		Short:        "Initialize a new registry",
		Args:         cobra.ExactArgs(2),
		RunE:         commands.RegistryInit, // Changed from Run to RunE
		SilenceUsage: true,                  // Prevent usage output in stderr
	}

	var registryCloneCmd = &cobra.Command{
		Use:          "clone [giturl]",
		Short:        "Clone a registry from a Git URL",
		Args:         cobra.ExactArgs(1),
		RunE:         commands.RegistryClone,
		SilenceUsage: true, // Prevent usage output in stderr
	}

	var registryDeleteCmd = &cobra.Command{
		Use:          "delete [registry-name]",
		Short:        "Delete a registry",
		Args:         cobra.ExactArgs(1),
		RunE:         commands.RegistryDelete,
		SilenceUsage: true, // Prevent usage output in stderr
	}
	registryDeleteCmd.Flags().BoolP("force", "f", false, "Force deletion of the registry")

	var registryUpdateCmd = &cobra.Command{
		Use:          "update [registry-name | --all]",
		Short:        "Update and synchronize a registry with its remote",
		Args:         cobra.MaximumNArgs(1),
		RunE:         commands.RegistryUpdate,
		SilenceUsage: true, // Prevent usage output in stderr
	}
	registryUpdateCmd.Flags().Bool("all", false, "Update all registries")

	var registryAddCmd = &cobra.Command{
		Use:   "add <registry name> <package giturl> | <registry name> <package name> <version>",
		Short: "Add a package or a specific version to a registry",
		Args:  cobra.RangeArgs(2, 3), // Allow 2 or 3 arguments
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.RegistryAdd(cmd, args)
		},
		SilenceUsage: true, // Prevent usage output in stderr
	}

	var registryRmCmd = &cobra.Command{
		Use:          "rm [registry-name] [package-name] [v<version>]",
		Short:        "Remove a package or version from a registry",
		Args:         cobra.RangeArgs(2, 3),
		RunE:         commands.RegistryRm,
		SilenceUsage: true, // Prevent usage output in stderr
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
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(releaseCmd)
	rootCmd.AddCommand(developCmd)
	rootCmd.AddCommand(freeCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(downgradeCmd)
	rootCmd.AddCommand(registryCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1) // Remove manual error printing, let Cobra handle it
	}
}
