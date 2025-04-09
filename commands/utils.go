package commands

import (
	"fmt"
	"os"
	"path/filepath"
)

const Version = "0.1.0" // Move the version constant here

// getGlobalCosmDir returns the global .cosm directory in the user's home directory
func getGlobalCosmDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %v", err)
	}
	return filepath.Join(homeDir, ".cosm"), nil
}

var ValidRegistries = []string{"cosmic-hub", "local"}

// PrintVersion prints the version of the cosm tool and exits
func PrintVersion() {
	fmt.Printf("cosm version %s\n", Version)
	os.Exit(0)
}
