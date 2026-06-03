package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRule_WithFrontmatter(t *testing.T) {
	content := []byte(`---
name: go-style
description: Go code style guidelines
file_pattern: "*.go"
global: false
---

Use gofumpt for formatting.
Always handle errors.
`)
	rule, err := ParseContent(content)
	require.NoError(t, err)
	require.Equal(t, "go-style", rule.Name)
	require.Equal(t, "Go code style guidelines", rule.Description)
	require.Equal(t, "*.go", rule.FilePattern)
	require.False(t, rule.Global)
	require.Contains(t, rule.Content, "gofumpt")
}

func TestParseRule_Global(t *testing.T) {
	content := []byte(`---
name: global-rule
description: Applies everywhere
global: true
---

This rule always applies.
`)
	rule, err := ParseContent(content)
	require.NoError(t, err)
	require.True(t, rule.Global)
}

func TestParseRule_NoFrontmatter(t *testing.T) {
	content := []byte(`Just some text without frontmatter`)
	_, err := ParseContent(content)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no YAML frontmatter found")
}

func TestParseRule_InvalidYAML(t *testing.T) {
	content := []byte(`---
name: 123
invalid_yaml: [:
---`)
	_, err := ParseContent(content)
	require.Error(t, err)
}

func TestParseRule_UnclosedFrontmatter(t *testing.T) {
	content := []byte(`---
name: test
description: no close
`)
	_, err := ParseContent(content)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unclosed frontmatter")
}

func TestValidateRule_MissingName(t *testing.T) {
	r := &Rule{Description: "test"}
	err := r.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is required")
}

func TestValidateRule_NameTooLong(t *testing.T) {
	name := ""
	for i := 0; i < MaxNameLength+1; i++ {
		name += "a"
	}
	r := &Rule{Name: name, Description: "test"}
	err := r.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "name exceeds")
}

func TestValidateRule_Valid(t *testing.T) {
	r := &Rule{Name: "my-rule", Description: "test rule"}
	err := r.Validate()
	require.NoError(t, err)
}

func TestParseFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-rule.mdc")
	err := os.WriteFile(path, []byte(`---
name: file-rule
description: Rule from file
file_pattern: "*.ts"
---

TypeScript rules here.
`), 0o644)
	require.NoError(t, err)

	rule, err := Parse(path)
	require.NoError(t, err)
	require.Equal(t, "file-rule", rule.Name)
	require.Equal(t, "*.ts", rule.FilePattern)
	require.Equal(t, dir, rule.Path)
	require.Equal(t, path, rule.FilePath)
}

func TestParseFromFile_DeriveName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "derived-name.mdc")
	err := os.WriteFile(path, []byte(`---
description: No name in frontmatter
---
Content
`), 0o644)
	require.NoError(t, err)

	rule, err := Parse(path)
	require.NoError(t, err)
	require.Equal(t, "derived-name", rule.Name)
}

func TestMatch_ExactGlob(t *testing.T) {
	rule := &Rule{Name: "test", FilePattern: "*.go"}
	require.True(t, Match("main.go", rule))
	require.False(t, Match("main.ts", rule))
}

func TestMatch_Global(t *testing.T) {
	rule := &Rule{Name: "test", Global: true}
	require.True(t, Match("any/file.txt", rule))
}

func TestMatch_EmptyPattern(t *testing.T) {
	rule := &Rule{Name: "test", FilePattern: ""}
	require.True(t, Match("anything", rule))
}

func TestMatch_Doublestar(t *testing.T) {
	rule := &Rule{Name: "test", FilePattern: "src/**/*.ts"}
	require.True(t, Match("src/components/Button.ts", rule))
	require.False(t, Match("src/main.go", rule))
	require.False(t, Match("test/Button.ts", rule))
}

func TestMatch_BaseFilename(t *testing.T) {
	rule := &Rule{Name: "test", FilePattern: "Dockerfile"}
	require.True(t, Match("some/path/Dockerfile", rule))
	require.False(t, Match("Dockerfile.build", rule))
}

func TestDiscover_EmptyPaths(t *testing.T) {
	rules := Discover(nil)
	require.Empty(t, rules)

	rules = Discover([]string{})
	require.Empty(t, rules)
}

func TestDiscover_NonExistentPath(t *testing.T) {
	rules := Discover([]string{"/nonexistent/path"})
	require.Empty(t, rules)
}

func TestDiscover_WithStates(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "valid.mdc"), []byte(`---
name: valid-rule
description: A valid rule
---
Content
`), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "invalid.mdc"), []byte(`No frontmatter`), 0o644)
	require.NoError(t, err)

	rules, states := DiscoverWithStates([]string{dir})
	require.Len(t, rules, 1)
	require.Equal(t, "valid-rule", rules[0].Name)

	// We should have at least one state entry
	require.NotEmpty(t, states)
}

func TestDeduplicate(t *testing.T) {
	rules := []*Rule{
		{Name: "duplicate", Description: "first", Path: "/global"},
		{Name: "unique", Description: "only one", Path: "/global"},
		{Name: "duplicate", Description: "second (should win)", Path: "/project"},
	}

	deduped := Deduplicate(rules)
	require.Len(t, deduped, 2)

	for _, r := range deduped {
		if r.Name == "duplicate" {
			require.Equal(t, "second (should win)", r.Description)
			require.Equal(t, "/project", r.Path)
		}
	}
}

func TestToPromptXML_Empty(t *testing.T) {
	require.Empty(t, ToPromptXML(nil))
	require.Empty(t, ToPromptXML([]*Rule{}))
}

func TestToPromptXML_WithRules(t *testing.T) {
	rules := []*Rule{
		{
			Name:        "test-rule",
			Description: "A test rule",
			FilePattern: "*.go",
			FilePath:    "/home/user/.agents/rules/test-rule.mdc",
		},
	}

	xml := ToPromptXML(rules)
	require.Contains(t, xml, "test-rule")
	require.Contains(t, xml, "A test rule")
	require.Contains(t, xml, "*.go")
	require.Contains(t, xml, "/home/user/.agents/rules/test-rule.mdc")
}

func TestToPromptContent(t *testing.T) {
	rules := []*Rule{
		{
			Name:        "test-rule",
			Description: "A test rule",
			Content:     "Always use tabs for indentation.",
		},
	}

	content := ToPromptContent(rules)
	require.Contains(t, content, "test-rule")
	require.Contains(t, content, "Always use tabs")
}

func TestToPromptContent_EmptyContent(t *testing.T) {
	rules := []*Rule{
		{Name: "empty", Content: ""},
	}
	content := ToPromptContent(rules)
	// The header is still emitted; only rules with content are included.
	require.Contains(t, content, "## Active Rules")
	require.NotContains(t, content, "empty")
}
