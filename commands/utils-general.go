package commands

import (
	"fmt"
	"strings"
)

const Version = "0.1.0" // Move the version constant here

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
