package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands ~ to home directory in a path string.
func ExpandPath(p string) (string, error) {
	if p == "" {
		return p, nil
	}

	if p == "~" {
		return os.UserHomeDir()
	}

	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		return filepath.Join(home, p[2:]), nil
	}

	return p, nil
}

// ExpandGlob expands a path with ~ and then expands glob patterns.
// Returns the list of matched files.
func ExpandGlob(pattern string) ([]string, error) {
	expanded, err := ExpandPath(pattern)
	if err != nil {
		return nil, err
	}

	matches, err := filepath.Glob(expanded)
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", expanded, err)
	}

	return matches, nil
}

// ContractPath replaces home directory with ~ in a path for display.
func ContractPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}

	if p == home {
		return "~"
	}

	if strings.HasPrefix(p, home+"/") {
		return "~/" + p[len(home)+1:]
	}

	return p
}
