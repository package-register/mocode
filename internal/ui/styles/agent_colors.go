package styles

import (
	"fmt"
	"hash/fnv"
	"image/color"
	"sync"

	"charm.land/lipgloss/v2"
)

// AgentBadgeColors holds the resolved foreground and background colors for an agent badge.
type AgentBadgeColors struct {
	Fg color.Color
	Bg color.Color
}

// Fallback colors used when an agent is not in the registry or the cache is empty.
var (
	fallbackBadgeColors = AgentBadgeColors{
		Fg: lipgloss.Color("#ffffff"),
		Bg: lipgloss.Color("#1D63ED"),
	}
	fallbackBadgeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#1D63ED")).Padding(0, 1)
	fallbackAccentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379"))
)

// cachedBadgeStyle holds a precomputed badge style for a palette index.
type cachedBadgeStyle struct {
	colors AgentBadgeColors
	style  lipgloss.Style
}

// agentRegistry maps agent names to their index in the team list and holds
// precomputed styles for each palette entry.
var agentRegistry struct {
	sync.RWMutex

	indices      map[string]int
	badgeStyles  []cachedBadgeStyle
	accentStyles []lipgloss.Style
	lastBgColor  color.Color // cached for invalidation checks
}

// SetBackgroundColor sets the background color used for agent badge generation.
// This should be called whenever the theme changes, before InvalidateAgentColorCache.
var agentBgColor color.Color

// SetAgentOrder updates the agent name to index mapping and rebuilds the style cache.
// Call this when the available agents change (e.g., on config reload).
func SetAgentOrder(agentNames []string) {
	agentRegistry.Lock()
	defer agentRegistry.Unlock()

	agentRegistry.indices = make(map[string]int, len(agentNames))
	for i, name := range agentNames {
		agentRegistry.indices[name] = i
	}

	rebuildAgentColorCache()
}

// rebuildAgentColorCache precomputes badge and accent styles from the current theme's hues.
func rebuildAgentColorCache() {
	hues := defaultAgentHues

	bg := getBackgroundColor()
	agentBgColor = bg

	badgeColors := generateBadgePalette(hues, bg)
	accentColors := generateAccentPalette(hues, bg)

	agentRegistry.badgeStyles = make([]cachedBadgeStyle, len(badgeColors))
	for i, bgColor := range badgeColors {
		r, g, b := colorToRGB(bgColor)
		fgHex := bestForegroundHex(
			rgbToHex(r, g, b),
			"#E5F2FC",
			"#1C1C22",
		)
		colors := AgentBadgeColors{
			Fg: lipgloss.Color(fgHex),
			Bg: bgColor,
		}
		agentRegistry.badgeStyles[i] = cachedBadgeStyle{
			colors: colors,
			style: lipgloss.NewStyle().
				Foreground(colors.Fg).
				Background(colors.Bg).
				Padding(0, 1),
		}
	}

	agentRegistry.accentStyles = make([]lipgloss.Style, len(accentColors))
	for i, c := range accentColors {
		agentRegistry.accentStyles[i] = lipgloss.NewStyle().Foreground(c)
	}
}

// InvalidateAgentColorCache rebuilds the cached agent styles.
// Call this after a theme change so colors are recalculated against the new background.
func InvalidateAgentColorCache() {
	agentRegistry.Lock()
	defer agentRegistry.Unlock()

	rebuildAgentColorCache()
}

// lookupAgentIndex returns the palette index for the given agent name.
func lookupAgentIndex(agentName string) (int, bool) {
	if idx, ok := agentRegistry.indices[agentName]; ok {
		return idx, true
	}
	if agentName == "" {
		return 0, false
	}
	return stableAgentIndex(agentName), true
}

func stableAgentIndex(agentName string) int {
	if len(defaultAgentHues) == 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(agentName))
	return int(h.Sum32() % uint32(len(defaultAgentHues)))
}

// AgentBadgeStyleFor returns a lipgloss badge style colored for the given agent.
func AgentBadgeStyleFor(agentName string) lipgloss.Style {
	agentRegistry.RLock()
	defer agentRegistry.RUnlock()

	idx, ok := lookupAgentIndex(agentName)
	if !ok || len(agentRegistry.badgeStyles) == 0 {
		return fallbackBadgeStyle
	}

	return agentRegistry.badgeStyles[idx%len(agentRegistry.badgeStyles)].style
}

// AgentAccentStyleFor returns a foreground-only style for agent names (used in sidebar).
func AgentAccentStyleFor(agentName string) lipgloss.Style {
	agentRegistry.RLock()
	defer agentRegistry.RUnlock()

	idx, ok := lookupAgentIndex(agentName)
	if !ok || len(agentRegistry.accentStyles) == 0 {
		return fallbackAccentStyle
	}

	return agentRegistry.accentStyles[idx%len(agentRegistry.accentStyles)]
}

// AgentBadgeColorsFor returns the badge foreground/background colors for a given agent name.
func AgentBadgeColorsFor(agentName string) AgentBadgeColors {
	agentRegistry.RLock()
	defer agentRegistry.RUnlock()

	idx, ok := lookupAgentIndex(agentName)
	if !ok || len(agentRegistry.badgeStyles) == 0 {
		return fallbackBadgeColors
	}

	return agentRegistry.badgeStyles[idx%len(agentRegistry.badgeStyles)].colors
}

// defaultAgentHues provides a default set of hue values.
var defaultAgentHues = []float64{
	200.0, // blue
	340.0, // pink
	80.0,  // green
	40.0,  // orange
	260.0, // purple
	160.0, // teal
	300.0, // magenta
	20.0,  // red
}

// getBackgroundColor returns the current theme background color, or a
// sensible dark default if the theme has not been initialized yet.
func getBackgroundColor() color.Color {
	if agentBgColor != nil {
		return agentBgColor
	}
	return lipgloss.Color("#1C1C22")
}

// generateBadgePalette generates background colours for agent badges from hue values.
func generateBadgePalette(hues []float64, bg color.Color) []color.Color {
	result := make([]color.Color, len(hues))
	bgR, bgG, bgB := colorToRGB(bg)
	for i, h := range hues {
		l := 0.35
		if bgR < 128 && bgG < 128 && bgB < 128 {
			l = 0.45
		}
		result[i] = hslToColor(h, 0.6, l)
	}
	return result
}

// generateAccentPalette generates foreground accent colours for agent names.
func generateAccentPalette(hues []float64, _ color.Color) []color.Color {
	result := make([]color.Color, len(hues))
	for i, h := range hues {
		result[i] = hslToColor(h, 0.7, 0.6)
	}
	return result
}

// bestForegroundHex returns the hex string of either lightText or darkText,
// whichever has better contrast against bgHex.
func bestForegroundHex(bgHex, lightText, darkText string) string {
	r, g, b := hexToRGB(bgHex)
	luminance := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if luminance > 128 {
		return darkText
	}
	return lightText
}

// hexToRGB converts a hex color string to RGB values.
func hexToRGB(hex string) (uint8, uint8, uint8) {
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) >= 6 {
		return parseHexPair(hex[0:2]), parseHexPair(hex[2:4]), parseHexPair(hex[4:6])
	}
	return 0, 0, 0
}

func parseHexPair(s string) uint8 {
	var v uint8
	for _, c := range s {
		v *= 16
		switch {
		case c >= '0' && c <= '9':
			v += uint8(c - '0')
		case c >= 'a' && c <= 'f':
			v += uint8(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			v += uint8(c - 'A' + 10)
		}
	}
	return v
}

func rgbToHex(r, g, b uint8) string {
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func colorToRGB(c color.Color) (uint8, uint8, uint8) {
	if c == nil {
		return 0, 0, 0
	}
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

// hslToColor converts HSL values to a color.Color.
func hslToColor(h, s, l float64) color.Color {
	h = clampFloat(h, 0, 360)
	s = clampFloat(s, 0, 1)
	l = clampFloat(l, 0, 1)

	c := (1 - absFloat(2*l-1)) * s
	x := c * (1 - absFloat(modFloat(h/60, 2)-1))
	m := l - c/2

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return color.RGBA{
		R: uint8((r + m) * 255),
		G: uint8((g + m) * 255),
		B: uint8((b + m) * 255),
		A: 255,
	}
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func modFloat(a, b float64) float64 {
	return a - b*float64(int64(a/b))
}
