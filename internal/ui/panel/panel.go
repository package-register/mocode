// Package panel provides tmux-like split-panel rendering for the UI.
// Panels can be split horizontally or vertically and arranged in a tree.
// Each panel holds content as a string and renders with borders and titles.
package panel

import (
	"image"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// Direction determines how a split is arranged.
type Direction int

const (
	Vertical   Direction = iota // left/right split
	Horizontal                  // top/bottom split
)

// Panel is a single pane in a split layout.
type Panel struct {
	ID      string
	Title   string
	Content string       // rendered content string
	mu      sync.RWMutex // protects Content

	// Split children. When non-empty, this panel is a container.
	Direction Direction
	Children  []*Panel
	Sizes     []float64 // proportion for each child (0-1, must sum to 1)

	// Style
	Border     lipgloss.Style
	TitleStyle lipgloss.Style
	Active     bool // highlighted border when focused
}

// New creates a new leaf panel.
func New(id, title string) *Panel {
	return &Panel{
		ID:         id,
		Title:      title,
		Border:     defaultBorder,
		TitleStyle: defaultTitle,
	}
}

// NewSplit creates a container panel with children.
func NewSplit(id string, dir Direction, children ...*Panel) *Panel {
	sizes := make([]float64, len(children))
	for i := range sizes {
		sizes[i] = 1.0 / float64(len(children))
	}
	return &Panel{
		ID:        id,
		Direction: dir,
		Children:  children,
		Sizes:     sizes,
		Border:    defaultBorder,
	}
}

// SetContent updates the panel's content (thread-safe).
func (p *Panel) SetContent(content string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Content = content
}

// GetContent returns the panel's content (thread-safe).
func (p *Panel) GetContent() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Content
}

// IsLeaf returns true if this panel has no children.
func (p *Panel) IsLeaf() bool {
	return len(p.Children) == 0
}

// Find returns the panel with the given ID, or nil.
func (p *Panel) Find(id string) *Panel {
	if p.ID == id {
		return p
	}
	for _, child := range p.Children {
		if found := child.Find(id); found != nil {
			return found
		}
	}
	return nil
}

// RenderAt draws the panel tree into the given screen region.
func (p *Panel) RenderAt(scr uv.Screen, area uv.Rectangle) {
	if p.IsLeaf() {
		p.renderLeaf(scr, area)
	} else {
		p.renderSplit(scr, area)
	}
}

func (p *Panel) renderLeaf(scr uv.Screen, area uv.Rectangle) {
	content := p.GetContent()

	// Apply border
	borderStyle := p.Border
	if p.Active {
		borderStyle = borderStyle.Foreground(lipgloss.Color("205")) // highlight active
	}

	// Build bordered view
	innerW := area.Dx() - 4 // 2 borders on each side
	innerH := area.Dy() - 2 // top + bottom border

	title := ""
	if p.Title != "" {
		title = p.TitleStyle.Render(" " + p.Title + " ")
	}

	// Pad content to fit
	lines := strings.Split(content, "\n")
	rendered := make([]string, 0, innerH)
	for i := 0; i < innerH; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		// Truncate or pad to innerW
		if len(line) > innerW {
			rendered = append(rendered, line[:innerW])
		} else {
			rendered = append(rendered, line+strings.Repeat(" ", innerW-len(line)))
		}
	}

	// Build full bordered view
	var b strings.Builder
	// Top border
	if title != "" && innerW > len(title)+2 {
		b.WriteString("┌")
		b.WriteString(title)
		b.WriteString(strings.Repeat("─", innerW-len(title)))
		b.WriteString("┐\n")
	} else {
		b.WriteString("┌" + strings.Repeat("─", innerW) + "┐\n")
	}
	for _, line := range rendered {
		b.WriteString("│" + line + "│\n")
	}
	b.WriteString("└" + strings.Repeat("─", innerW) + "┘")

	bordered := borderStyle.Render(b.String())
	uv.NewStyledString(bordered).Draw(scr, area)
}

func (p *Panel) renderSplit(scr uv.Screen, area uv.Rectangle) {
	if len(p.Children) == 0 {
		return
	}

	var offset int
	for i, child := range p.Children {
		var childArea uv.Rectangle
		switch p.Direction {
		case Vertical:
			childW := int(float64(area.Dx()) * p.Sizes[i])
			if i == len(p.Children)-1 {
				childW = area.Dx() - offset // use remaining space
			}
			childArea = uv.Rectangle{
				Min: image.Point{X: area.Min.X + offset, Y: area.Min.Y},
				Max: image.Point{X: area.Min.X + offset + childW, Y: area.Max.Y},
			}
			offset += childW
		case Horizontal:
			childH := int(float64(area.Dy()) * p.Sizes[i])
			if i == len(p.Children)-1 {
				childH = area.Dy() - offset // use remaining space
			}
			childArea = uv.Rectangle{
				Min: image.Point{X: area.Min.X, Y: area.Min.Y + offset},
				Max: image.Point{X: area.Max.X, Y: area.Min.Y + offset + childH},
			}
			offset += childH
		}
		child.RenderAt(scr, childArea)
	}
}

// --- Default styles ---

var defaultBorder = lipgloss.NewStyle().
	Foreground(lipgloss.Color("240"))

var defaultTitle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("252")).
	Background(lipgloss.Color("236")).
	Bold(true)
