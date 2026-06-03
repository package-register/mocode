package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteTrailingMention(t *testing.T) {
	t.Parallel()

	t.Run("deletes trailing workflow mention", func(t *testing.T) {
		t.Parallel()

		got, ok := deleteTrailingMention("@workflow:windsurf/flash-card ")
		require.True(t, ok)
		require.Equal(t, " ", got)
	})

	t.Run("deletes only final mention and preserves prefix text", func(t *testing.T) {
		t.Parallel()

		got, ok := deleteTrailingMention("please use @file:README.md ")
		require.True(t, ok)
		require.Equal(t, "please use ", got)
	})

	t.Run("ignores plain text", func(t *testing.T) {
		t.Parallel()

		got, ok := deleteTrailingMention("please use readme ")
		require.False(t, ok)
		require.Equal(t, "please use readme ", got)
	})
}
