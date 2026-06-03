package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestEditorKeyReplacesSelection(t *testing.T) {
	t.Parallel()

	require.True(t, editorKeyReplacesSelection(tea.KeyPressMsg{Text: "a"}))
	require.True(t, editorKeyReplacesSelection(tea.KeyPressMsg{Code: tea.KeySpace}))
	require.True(t, editorKeyReplacesSelection(tea.KeyPressMsg{Code: tea.KeyBackspace}))
	require.False(t, editorKeyReplacesSelection(tea.KeyPressMsg{Code: tea.KeyLeft}))
	require.False(t, editorKeyReplacesSelection(tea.KeyPressMsg{Text: "ctrl+c"}))
}
