package model

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkflowFileDeduplicationByToken(t *testing.T) {
	t.Parallel()

	seenPaths := map[string]bool{}
	seenTokens := map[string]bool{}
	items := []string{}

	add := func(path, rel string) {
		name := strings.TrimSuffix(normalizeContextPath(rel), filepath.Ext(rel))
		token := "@workflow:windsurf/" + name
		if name == "" || seenPaths[path] || seenTokens[token] {
			return
		}
		seenPaths[path] = true
		seenTokens[token] = true
		items = append(items, token)
	}

	add(filepath.Join("local", "flash-card.md"), "flash-card.md")
	add(filepath.Join("home", "flash-card.md"), "flash-card.md")
	add(filepath.Join("config", "other.md"), "other.md")

	require.Equal(t, []string{"@workflow:windsurf/flash-card", "@workflow:windsurf/other"}, items)
}
