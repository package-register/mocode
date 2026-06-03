package rules

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/charlievieth/fastwalk"
)

// Discover finds all valid rules in the given paths.
func Discover(paths []string) []*Rule {
	rules, _ := DiscoverWithStates(paths)
	return rules
}

// DiscoverWithStates finds all valid rules and returns per-file diagnostic states.
func DiscoverWithStates(paths []string) ([]*Rule, []*RuleState) {
	var rules []*Rule
	var states []*RuleState
	var mu sync.Mutex
	seen := make(map[string]bool)

	addState := func(name, path string, state DiscoveryState, err error) {
		mu.Lock()
		states = append(states, &RuleState{
			Name:  name,
			Path:  path,
			State: state,
			Err:   err,
		})
		mu.Unlock()
	}

	for _, base := range paths {
		conf := fastwalk.Config{
			Follow:  true,
			ToSlash: fastwalk.DefaultToSlash(),
		}
		err := fastwalk.Walk(&conf, base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				slog.Warn("Failed to walk rules path entry", "base", base, "path", path, "error", err)
				addState("", path, StateError, err)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".md" && ext != ".mdc" {
				return nil
			}
			mu.Lock()
			if seen[path] {
				mu.Unlock()
				return nil
			}
			seen[path] = true
			mu.Unlock()

			rule, parseErr := Parse(path)
			if parseErr != nil {
				slog.Warn("Failed to parse rule file", "path", path, "error", parseErr)
				addState("", path, StateError, parseErr)
				return nil
			}
			if validateErr := rule.Validate(); validateErr != nil {
				slog.Warn("Rule validation failed", "path", path, "error", validateErr)
				addState(rule.Name, path, StateError, validateErr)
				return nil
			}
			slog.Debug("Successfully loaded rule", "name", rule.Name, "path", path)
			mu.Lock()
			rules = append(rules, rule)
			mu.Unlock()
			addState(rule.Name, path, StateNormal, nil)
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to walk rules path", "path", base, "error", err)
		}
	}

	sort.SliceStable(rules, func(i, j int) bool {
		return strings.ToLower(rules[i].FilePath) < strings.ToLower(rules[j].FilePath)
	})

	return rules, states
}

// DiscoveryState represents the outcome of discovering a single rule file.
type DiscoveryState int

const (
	// StateNormal indicates the rule was parsed and validated successfully.
	StateNormal DiscoveryState = iota
	// StateError indicates discovery encountered a scan/parse/validate error.
	StateError
)

// RuleState represents the latest discovery status of a rule file.
type RuleState struct {
	Name  string
	Path  string
	State DiscoveryState
	Err   error
}

// Deduplicate removes duplicate rules by name. The last occurrence wins,
// meaning project-level rules override global-level rules with the same name.
func Deduplicate(all []*Rule) []*Rule {
	seen := make(map[string]int, len(all))
	for i, r := range all {
		seen[r.Name] = i
	}

	result := make([]*Rule, 0, len(seen))
	for i, r := range all {
		if seen[r.Name] == i {
			result = append(result, r)
		}
	}
	return result
}
