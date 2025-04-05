package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

// cosm registry status <registry name>
// cosm registry init <registry name> <giturl>
// cosm registry clone <giturl>
// cosm registry delete <registry name> [--force]
// cosm registry update <registry name>
// cosm registry update --all

func main() {
	// Root command
	var rootCmd = &cobra.Command{
		Use:   "cosm",
		Short: "A cosmic package manager",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Welcome to Cosm! Use a subcommand like 'status' or 'registry'.")
		},
	}

	// Global --version flag
	var versionFlag bool
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print the version number")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if versionFlag {
			fmt.Printf("cosm version %s\n", version)
			os.Exit(0)
		}
	}

	// "status" subcommand (root-level)
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

	// "registry" parent command
	var registryCmd = &cobra.Command{
		Use:   "registry",
		Short: "Manage package registries",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Registry command requires a subcommand (e.g., 'status').")
		},
	}

	// "registry status <registry name>" subcommand
	var registryStatusCmd = &cobra.Command{
		Use:   "status [registry-name]",
		Short: "Show contents of a registry",
		Args:  cobra.ExactArgs(1), // Requires exactly one argument (registry name)
		Run: func(cmd *cobra.Command, args []string) {
			registryName := args[0]
			fmt.Printf("Status for registry '%s':\n", registryName)
			fmt.Println("  Available packages:")
			fmt.Printf("    - %s-pkg1 (v1.0.0)\n", registryName)
			fmt.Printf("    - %s-pkg2 (v2.1.3)\n", registryName)
			fmt.Println("  Last updated: 2025-04-05")
		},
	}

	// Add subcommand to registry
	registryCmd.AddCommand(registryStatusCmd)

	// Add commands to root
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(registryCmd)

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
