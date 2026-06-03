// Package merge provides a generic Merge[T] function for combining slices of
// values into one.
//
// Inspired by trpc-agent-go/tool/merge.go, but with the reflect-heavy
// struct/map/array dispatch removed. mocode only needs the patterns that are
// actually useful in the tool decorator stack: strings, byte slices, and any
// custom type that opts in via the Mergeable interface.
//
// Supported merge strategies:
//   - string  → concatenation
//   - []byte  → append
//   - Mergeable → delegated to the type's own Merge method
//   - anything else → return the first element (identity)
//
// Usage:
//
//	full := merge.Merge([]string{"hello ", "world"})  // → "hello world"
//
//	chunks := []MyChunk{a, b, c}
//	combined := merge.Merge(chunks)  // MyChunk must implement Mergeable
package merge

import "strings"

// Mergeable is implemented by types that define their own merging strategy.
// Merge is called left-to-right: result = result.Merge(next).
// Inspired by trpc-agent-go/tool/merge.go.
type Mergeable interface {
	Merge(other any) any
}

// Merge combines a slice of T into a single value.
//
//   - Empty slice  → zero value of T
//   - Single item  → that item unchanged
//   - []string     → concatenation (same as MergeStrings)
//   - [][]byte     → append (same as MergeBytes)
//   - Mergeable    → left-fold via Merge method
//   - Anything else → first element
func Merge[T any](ts []T) T {
	var zero T
	switch len(ts) {
	case 0:
		return zero
	case 1:
		return ts[0]
	}

	// string shortcut
	if _, ok := any(ts[0]).(string); ok {
		return mergeStrings(ts)
	}

	// []byte shortcut
	if _, ok := any(ts[0]).([]byte); ok {
		return mergeBytes(ts)
	}

	// Mergeable delegation
	if _, ok := any(ts[0]).(Mergeable); ok {
		result := any(ts[0])
		for i := 1; i < len(ts); i++ {
			if m, ok := result.(Mergeable); ok {
				result = m.Merge(ts[i])
			}
		}
		if v, ok := result.(T); ok {
			return v
		}
		return zero
	}

	// Fallback: return first element unchanged.
	return ts[0]
}

// MergeStrings concatenates all strings in ss.
func MergeStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	var b strings.Builder
	for _, s := range ss {
		b.WriteString(s)
	}
	return b.String()
}

// MergeBytes appends all byte slices in bs.
func MergeBytes(bs [][]byte) []byte {
	if len(bs) == 0 {
		return nil
	}
	var total int
	for _, b := range bs {
		total += len(b)
	}
	out := make([]byte, 0, total)
	for _, b := range bs {
		out = append(out, b...)
	}
	return out
}

// mergeStrings is the internal generic implementation used by Merge[string].
func mergeStrings[T any](ts []T) T {
	var b strings.Builder
	for _, t := range ts {
		if s, ok := any(t).(string); ok {
			b.WriteString(s)
		}
	}
	result := any(b.String())
	if v, ok := result.(T); ok {
		return v
	}
	var zero T
	return zero
}

// mergeBytes is the internal generic implementation used by Merge[[]byte].
func mergeBytes[T any](ts []T) T {
	var out []byte
	for _, t := range ts {
		if b, ok := any(t).([]byte); ok {
			out = append(out, b...)
		}
	}
	result := any(out)
	if v, ok := result.(T); ok {
		return v
	}
	var zero T
	return zero
}
