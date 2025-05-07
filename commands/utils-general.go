package commands

import (
	"fmt"
	"os/exec"
	"strings"
)

// runCommand executes a command in the specified directory, returning the output and any error.
// The command is provided as a slice of arguments (e.g., []string{"git", "checkout", "-"}).
func runCommand(dir string, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("no command arguments provided")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	if err != nil {
		return outputStr, fmt.Errorf("failed to run '%s' in %s: %v\nOutput: %s", strings.Join(args, " "), dir, err, outputStr)
	}
	return outputStr, nil
}

// removeString removes the specified string from a slice of strings
func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// promptUserForConfirmation prompts the user for confirmation and returns true if they enter 'y' or 'Y'
func promptUserForConfirmation(prompt string) bool {
	fmt.Print(prompt)
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false
	}
	return strings.ToLower(response) == "y"
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
