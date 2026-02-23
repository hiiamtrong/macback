package crypto

import (
	"path/filepath"
	"strings"
)

// IsSecret checks if a filename matches any secret pattern.
// Patterns use filepath.Match syntax. Negation patterns start with "!".
// A negation pattern excludes the file from being considered a secret.
func IsSecret(filename string, categoryPatterns, globalPatterns []string) bool {
	baseName := filepath.Base(filename)

	// Check category patterns first
	matched := matchPatterns(baseName, categoryPatterns)

	// If not matched by category patterns, check global patterns
	if !matched {
		matched = matchPatterns(baseName, globalPatterns)
	}

	return matched
}

// matchPatterns checks if a name matches any pattern in the list.
// Negation patterns (starting with "!") take priority - if any negation matches,
// the file is NOT a secret even if positive patterns match.
func matchPatterns(name string, patterns []string) bool {
	hasPositiveMatch := false
	hasNegativeMatch := false

	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") {
			// Negation pattern
			negPattern := pattern[1:]
			if matched, _ := filepath.Match(negPattern, name); matched {
				hasNegativeMatch = true
			}
		} else {
			// Positive pattern
			if matched, _ := filepath.Match(pattern, name); matched {
				hasPositiveMatch = true
			}
		}
	}

	// Negation takes priority
	if hasNegativeMatch {
		return false
	}

	return hasPositiveMatch
}
