package merge_test

import (
	"testing"

	"github.com/package-register/mocode/internal/agent/tools/internal/merge"
	"github.com/stretchr/testify/assert"
)

// ─── Merge[string] ────────────────────────────────────────────────────────────

func TestMerge_String_Empty_ReturnsZero(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", merge.Merge([]string{}))
}

func TestMerge_String_Single_ReturnsIdentity(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "hello", merge.Merge([]string{"hello"}))
}

func TestMerge_String_Multiple_Concatenates(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abc", merge.Merge([]string{"a", "b", "c"}))
}

// ─── MergeStrings helper ──────────────────────────────────────────────────────

func TestMergeStrings_JoinsAll(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "hello world", merge.MergeStrings([]string{"hello ", "world"}))
}

func TestMergeStrings_Empty_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", merge.MergeStrings(nil))
}

// ─── Merge[[]byte] ───────────────────────────────────────────────────────────

func TestMerge_Bytes_Empty_ReturnsNil(t *testing.T) {
	t.Parallel()
	got := merge.Merge([][]byte{})
	assert.Nil(t, got)
}

func TestMerge_Bytes_Multiple_Appends(t *testing.T) {
	t.Parallel()
	got := merge.Merge([][]byte{{'a', 'b'}, {'c', 'd'}})
	assert.Equal(t, []byte("abcd"), got)
}

// ─── MergeBytes helper ────────────────────────────────────────────────────────

func TestMergeBytes_JoinsAll(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []byte("xyz"), merge.MergeBytes([][]byte{{'x'}, {'y'}, {'z'}}))
}

func TestMergeBytes_Nil_ReturnsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, merge.MergeBytes(nil))
}

// ─── Merge with Mergeable custom type ────────────────────────────────────────

type accum struct{ total int }

func (a accum) Merge(other any) any {
	if b, ok := other.(accum); ok {
		return accum{total: a.total + b.total}
	}
	return a
}

func TestMerge_Mergeable_Custom_Accumulates(t *testing.T) {
	t.Parallel()
	items := []accum{{1}, {2}, {3}}
	got := merge.Merge(items)
	assert.Equal(t, accum{total: 6}, got)
}

func TestMerge_Mergeable_Single_ReturnsIdentity(t *testing.T) {
	t.Parallel()
	got := merge.Merge([]accum{{42}})
	assert.Equal(t, accum{total: 42}, got)
}

// ─── Nil / empty edge cases ───────────────────────────────────────────────────

func TestMerge_Int_Empty_ReturnsZero(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0, merge.Merge([]int{}))
}

func TestMerge_Int_Single_ReturnsValue(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 7, merge.Merge([]int{7}))
}

func TestMerge_Int_Multiple_ReturnsFirst(t *testing.T) {
	t.Parallel()
	// For non-string, non-byte, non-Mergeable types, Merge returns the first element.
	assert.Equal(t, 1, merge.Merge([]int{1, 2, 3}))
}
