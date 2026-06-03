package rules

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Match checks whether the given file path matches the rule's pattern.
// If the rule has Global set to true, it always matches.
// The filePath should be relative to the project root.
func Match(filePath string, rule *Rule) bool {
	if rule.Global {
		return true
	}
	if rule.FilePattern == "" {
		return true
	}

	// Clean the path for consistent matching.
	filePath = filepath.ToSlash(filePath)
	pattern := filepath.ToSlash(rule.FilePattern)

	// Try exact match first.
	if matched, err := filepath.Match(pattern, filePath); err == nil && matched {
		return true
	}

	// Try doublestar (recursive glob) matching.
	if matched, err := doublestar.Match(pattern, filePath); err == nil && matched {
		return true
	}

	// If the pattern doesn't contain path separators, also match just the
	// base filename.
	if !strings.Contains(pattern, "/") {
		base := filepath.Base(filePath)
		if matched, err := filepath.Match(pattern, base); err == nil && matched {
			return true
		}
		if matched, err := doublestar.Match(pattern, base); err == nil && matched {
			return true
		}
	}

	return false
}
