// Package workflows provides workflow definition discovery, parsing, and
// todo-driven progression tracking.
//
// Workflows are Markdown files with YAML frontmatter placed in configured
// directories (e.g. ~/.agents/workflows/ or ./.agents/workflows/).
// Each workflow defines a sequence of steps that an agent should follow.
package workflows

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charlievieth/fastwalk"
	"gopkg.in/yaml.v3"
)

const MaxNameLength = 128

// Workflow represents a parsed workflow definition.
type Workflow struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Steps       []Step `yaml:"steps"`

	Path     string `yaml:"-"` // directory containing the file
	FilePath string `yaml:"-"` // full path to the file
}

// Step represents a single step in a workflow.
type Step struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

// Validate checks if the workflow is valid.
func (w *Workflow) Validate() error {
	var errs []error
	if w.Name == "" {
		errs = append(errs, errors.New("name is required"))
	} else if len(w.Name) > MaxNameLength {
		errs = append(errs, fmt.Errorf("name exceeds %d characters", MaxNameLength))
	}
	if len(w.Steps) == 0 {
		errs = append(errs, errors.New("at least one step is required"))
	}
	for i, s := range w.Steps {
		if s.ID == "" {
			errs = append(errs, fmt.Errorf("step %d: id is required", i))
		}
		if s.Title == "" {
			errs = append(errs, fmt.Errorf("step %d: title is required", i))
		}
	}
	return errors.Join(errs...)
}

// Parse parses a workflow file from disk.
func Parse(path string) (*Workflow, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	wf, err := ParseContent(content)
	if err != nil {
		return nil, err
	}

	wf.Path = filepath.Dir(path)
	wf.FilePath = path

	if wf.Name == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		wf.Name = base
	}

	return wf, nil
}

// ParseContent parses workflow content from raw bytes.
func ParseContent(content []byte) (*Workflow, error) {
	frontmatter, body, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	var wf Workflow
	if err := yaml.Unmarshal([]byte(frontmatter), &wf); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	// If steps have no description, fill from body sections.
	if wf.Steps == nil {
		wf.Steps = parseStepsFromBody(body)
	}

	return &wf, nil
}

// Discover finds all valid workflows in the given paths.
func Discover(paths []string) []*Workflow {
	var workflows []*Workflow
	var mu sync.Mutex
	seen := make(map[string]bool)

	for _, base := range paths {
		conf := fastwalk.Config{
			Follow:  true,
			ToSlash: fastwalk.DefaultToSlash(),
		}
		err := fastwalk.Walk(&conf, base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
				return nil
			}
			mu.Lock()
			if seen[path] {
				mu.Unlock()
				return nil
			}
			seen[path] = true
			mu.Unlock()

			wf, parseErr := Parse(path)
			if parseErr != nil {
				slog.Warn("Failed to parse workflow", "path", path, "error", parseErr)
				return nil
			}
			if validateErr := wf.Validate(); validateErr != nil {
				slog.Warn("Workflow validation failed", "path", path, "error", validateErr)
				return nil
			}
			slog.Debug("Loaded workflow", "name", wf.Name, "steps", len(wf.Steps))
			mu.Lock()
			workflows = append(workflows, wf)
			mu.Unlock()
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to walk workflows path", "path", base, "error", err)
		}
	}

	sort.SliceStable(workflows, func(i, j int) bool {
		return strings.ToLower(workflows[i].Name) < strings.ToLower(workflows[j].Name)
	})

	return workflows
}

func splitFrontmatter(content string) (string, string, error) {
	content = strings.TrimPrefix(content, "\uFEFF")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			start = i
			break
		}
	}
	if start == -1 || strings.TrimSpace(lines[start]) != "---" {
		return "", "", errors.New("no YAML frontmatter found")
	}

	endOffset := 0
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endOffset = i - start - 1
			break
		}
	}
	if endOffset == 0 {
		return "", "", errors.New("unclosed frontmatter")
	}
	end := start + 1 + endOffset

	frontmatter := strings.Join(lines[start+1:end], "\n")
	body := strings.Join(lines[end+1:], "\n")
	return frontmatter, body, nil
}

func parseStepsFromBody(body string) []Step {
	var steps []Step
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Match "- [ ] Title: Description" or "- [x] Title"
		if len(line) < 6 || line[:3] != "- [" {
			continue
		}
		content := strings.TrimSpace(line[5:]) // after "- [ ] " or "- [x] "
		title, description, _ := strings.Cut(content, ": ")
		if title == "" {
			continue
		}
		id := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
		steps = append(steps, Step{
			ID:          id,
			Title:       title,
			Description: strings.TrimSpace(description),
		})
	}
	return steps
}

// Now returns the current time. Exported for testing.
var Now = time.Now
