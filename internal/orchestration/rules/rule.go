// Package rules provides Cursor-style rule file discovery, matching, and
// system prompt injection.
//
// Rules are Markdown files with YAML frontmatter placed in configured
// directories (e.g. ~/.agents/rules/ or ./.agents/rules/).
package rules

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// MaxNameLength is the maximum allowed rule name length.
	MaxNameLength = 128
	// MaxDescriptionLength is the maximum allowed rule description length.
	MaxDescriptionLength = 1024
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

// Rule represents a single rule file parsed from a .md or .mdc file with
// YAML frontmatter.
type Rule struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	// FilePattern is a glob pattern that determines which files this rule
	// applies to (e.g. "*.go", "src/**/*.ts").
	FilePattern string `yaml:"file_pattern"`
	// Global, when true, means this rule applies regardless of the file
	// being viewed/edited.
	Global bool `yaml:"global"`

	// Path is the directory containing the rule file.
	Path string `yaml:"-"`
	// FilePath is the full path to the rule file.
	FilePath string `yaml:"-"`
	// Content is the rule body (the part after the YAML frontmatter).
	Content string `yaml:"-"`
}

// Validate checks if the rule meets specification requirements.
func (r *Rule) Validate() error {
	var errs []error

	if r.Name == "" {
		errs = append(errs, errors.New("name is required"))
	} else if len(r.Name) > MaxNameLength {
		errs = append(errs, fmt.Errorf("name exceeds %d characters", MaxNameLength))
	} else if !namePattern.MatchString(r.Name) {
		errs = append(errs, errors.New("name must be alphanumeric with hyphens, no leading/trailing/consecutive hyphens"))
	}

	if len(r.Description) > MaxDescriptionLength {
		errs = append(errs, fmt.Errorf("description exceeds %d characters", MaxDescriptionLength))
	}

	return errors.Join(errs...)
}

// Parse parses a rule file from disk. The file should have YAML frontmatter
// delimited by "---" lines, followed by markdown content.
func Parse(path string) (*Rule, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	rule, err := ParseContent(content)
	if err != nil {
		return nil, err
	}

	rule.Path = filepath.Dir(path)
	rule.FilePath = path

	// Derive name from filename if not set in frontmatter.
	if rule.Name == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		rule.Name = base
	}

	return rule, nil
}

// ParseContent parses rule content from raw bytes.
func ParseContent(content []byte) (*Rule, error) {
	frontmatter, body, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	var rule Rule
	if err := yaml.Unmarshal([]byte(frontmatter), &rule); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	rule.Content = strings.TrimSpace(body)

	return &rule, nil
}

// splitFrontmatter extracts YAML frontmatter and body from markdown content.
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	// Strip UTF-8 BOM for compatibility with editors that include it.
	content = strings.TrimPrefix(content, "\uFEFF")
	// Normalize line endings to \n for consistent parsing.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	lines := strings.Split(content, "\n")
	start := indexOfNonEmpty(lines)
	if start == -1 || strings.TrimSpace(lines[start]) != "---" {
		return "", "", errors.New("no YAML frontmatter found")
	}

	endOffset := indexFrom(lines[start+1:], func(line string) bool {
		return strings.TrimSpace(line) == "---"
	})
	if endOffset == -1 {
		return "", "", errors.New("unclosed frontmatter")
	}
	end := start + 1 + endOffset

	frontmatter = strings.Join(lines[start+1:end], "\n")
	body = strings.Join(lines[end+1:], "\n")
	return frontmatter, body, nil
}

func indexOfNonEmpty(lines []string) int {
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			return i
		}
	}
	return -1
}

func indexFrom(lines []string, fn func(string) bool) int {
	for i, line := range lines {
		if fn(line) {
			return i
		}
	}
	return -1
}
