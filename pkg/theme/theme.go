package theme

import (
	"fmt"
	"strings"
	"sync"
)

type Theme struct {
	Name         string
	DisplayName  string
	Primary      string // Main brand / highlight
	Secondary    string // Sub-headers / drivers
	Accent       string // Keywords / badges
	Success      string // Status online / ok
	Warning      string // Latency / warnings
	Error        string // Errors
	Muted        string // Borders / timestamps / separators
	UserPrompt   string // User prompt prefix
	AgentPrompt  string // Agent prompt prefix
	MemoryTag    string // Palace-Mnemosyne tags
	TelemetryTag string // Context / token gauges
	Reset        string // ANSI Reset
}

const Reset = "\033[0m"

type ThemeEngine struct {
	themes       map[string]*Theme
	activeTheme  *Theme
	colorEnabled bool
	mu           sync.RWMutex
}

var defaultEngine *ThemeEngine
var once sync.Once

func GetThemeEngine() *ThemeEngine {
	once.Do(func() {
		defaultEngine = NewThemeEngine("kinetic")
	})
	return defaultEngine
}

func NewThemeEngine(initialTheme string) *ThemeEngine {
	te := &ThemeEngine{
		themes:       make(map[string]*Theme),
		colorEnabled: true,
	}

	te.registerThemes()
	te.SetTheme(initialTheme)
	return te
}

func (te *ThemeEngine) registerThemes() {
	// 1. Kinetic Cyberpunk (Official Website Theme)
	te.themes["kinetic"] = &Theme{
		Name:         "kinetic",
		DisplayName:  "⚡ Kinetic Cyberpunk (Neon Cyan & Electric Violet)",
		Primary:      "\033[38;2;168;85;247m",  // Electric Purple #a855f7
		Secondary:    "\033[38;2;34;211;238m",  // Neon Cyan #22d3ee
		Accent:       "\033[38;2;232;121;249m", // Fuchsia Pink #e879f9
		Success:      "\033[38;2;52;211;153m",  // Emerald #34d399
		Warning:      "\033[38;2;251;191;36m",  // Amber #fbbf24
		Error:        "\033[38;2;248;113;113m", // Coral Red #f87171
		Muted:        "\033[38;2;107;114;128m", // Cool Gray #6b7280
		UserPrompt:   "\033[38;2;34;211;238;1m",
		AgentPrompt:  "\033[38;2;168;85;247;1m",
		MemoryTag:    "\033[38;2;232;121;249m",
		TelemetryTag: "\033[38;2;52;211;153m",
		Reset:        Reset,
	}

	// 2. Dracula Theme
	te.themes["dracula"] = &Theme{
		Name:         "dracula",
		DisplayName:  "🧛 Dracula (Gothic Violet, Pink & Cyan)",
		Primary:      "\033[38;2;189;147;249m", // Purple #bd93f9
		Secondary:    "\033[38;2;139;233;253m", // Cyan #8be9fd
		Accent:       "\033[38;2;255;121;198m", // Pink #ff79c6
		Success:      "\033[38;2;80;250;123m",  // Green #50fa7b
		Warning:      "\033[38;2;241;250;140m", // Yellow #f1fa8c
		Error:        "\033[38;2;255;85;85m",   // Red #ff5555
		Muted:        "\033[38;2;98;114;164m",  // Comment Gray #6272a4
		UserPrompt:   "\033[38;2;139;233;253;1m",
		AgentPrompt:  "\033[38;2;255;121;198;1m",
		MemoryTag:    "\033[38;2;189;147;249m",
		TelemetryTag: "\033[38;2;80;250;123m",
		Reset:        Reset,
	}

	// 3. Matrix Hacker Theme
	te.themes["matrix"] = &Theme{
		Name:         "matrix",
		DisplayName:  "🕶️ Matrix (Phosphor Green & Terminal Black)",
		Primary:      "\033[38;2;34;197;94m",   // Bright Green #22c55e
		Secondary:    "\033[38;2;74;222;128m",  // Mint #4ade80
		Accent:       "\033[38;2;187;247;208m", // Light Phosphor #bbf7d0
		Success:      "\033[38;2;34;197;94m",
		Warning:      "\033[38;2;234;179;8m",
		Error:        "\033[38;2;239;68;68m",
		Muted:        "\033[38;2;75;85;99m",
		UserPrompt:   "\033[38;2;74;222;128;1m",
		AgentPrompt:  "\033[38;2;34;197;94;1m",
		MemoryTag:    "\033[38;2;187;247;208m",
		TelemetryTag: "\033[38;2;34;197;94m",
		Reset:        Reset,
	}

	// 4. Catppuccin Mocha Theme
	te.themes["catppuccin"] = &Theme{
		Name:         "catppuccin",
		DisplayName:  "🐱 Catppuccin Mocha (Mauve, Peach & Lavender)",
		Primary:      "\033[38;2;203;166;247m", // Mauve #cba6f7
		Secondary:    "\033[38;2;137;180;250m", // Blue #89b4fa
		Accent:       "\033[38;2;250;179;135m", // Peach #fab387
		Success:      "\033[38;2;166;227;161m", // Green #a6e3a1
		Warning:      "\033[38;2;249;226;175m", // Yellow #f9e2af
		Error:        "\033[38;2;243;139;168m", // Red #f38ba8
		Muted:        "\033[38;2;108;112;134m", // Overlay0 #6c7086
		UserPrompt:   "\033[38;2;137;180;250;1m",
		AgentPrompt:  "\033[38;2;203;166;247;1m",
		MemoryTag:    "\033[38;2;250;179;135m",
		TelemetryTag: "\033[38;2;166;227;161m",
		Reset:        Reset,
	}

	// 5. Nord Frost Theme
	te.themes["nord"] = &Theme{
		Name:         "nord",
		DisplayName:  "❄️ Nord (Polar Frost Cyan & Glacier White)",
		Primary:      "\033[38;2;136;192;208m", // Frost Cyan #88c0d0
		Secondary:    "\033[38;2;129;161;193m", // Frost Blue #81a1c1
		Accent:       "\033[38;2;143;188;187m", // Frost Teal #8fbcbb
		Success:      "\033[38;2;163;190;140m", // Aurora Green #a3be8c
		Warning:      "\033[38;2;235;203;139m", // Aurora Yellow #ebcb8b
		Error:        "\033[38;2;191;97;106m",  // Aurora Red #bf616a
		Muted:        "\033[38;2;94;129;172m",  // Muted Blue #5e81ac
		UserPrompt:   "\033[38;2;136;192;208;1m",
		AgentPrompt:  "\033[38;2;143;188;187;1m",
		MemoryTag:    "\033[38;2;129;161;193m",
		TelemetryTag: "\033[38;2;163;190;140m",
		Reset:        Reset,
	}

	// 6. Retro Amber CRT Theme
	te.themes["amber"] = &Theme{
		Name:         "amber",
		DisplayName:  "📻 Retro Amber CRT (Vintage 1980s Terminal)",
		Primary:      "\033[38;2;255;176;0m", // Vintage Amber #ffb000
		Secondary:    "\033[38;2;255;204;0m", // Bright Amber #ffcc00
		Accent:       "\033[38;2;255;136;0m", // Dark Amber #ff8800
		Success:      "\033[38;2;255;204;0m",
		Warning:      "\033[38;2;255;176;0m",
		Error:        "\033[38;2;255;50;50m",
		Muted:        "\033[38;2;140;90;0m",
		UserPrompt:   "\033[38;2;255;204;0;1m",
		AgentPrompt:  "\033[38;2;255;176;0;1m",
		MemoryTag:    "\033[38;2;255;136;0m",
		TelemetryTag: "\033[38;2;255;204;0m",
		Reset:        Reset,
	}
}

func (te *ThemeEngine) SetTheme(name string) bool {
	te.mu.Lock()
	defer te.mu.Unlock()

	clean := strings.ToLower(strings.TrimSpace(name))
	if t, exists := te.themes[clean]; exists {
		te.activeTheme = t
		return true
	}
	// Default fallback
	if t, ok := te.themes["kinetic"]; ok {
		te.activeTheme = t
	}
	return false
}

func (te *ThemeEngine) Active() *Theme {
	te.mu.RLock()
	defer te.mu.RUnlock()
	if te.activeTheme == nil {
		return te.themes["kinetic"]
	}
	return te.activeTheme
}

func (te *ThemeEngine) ListThemes() []*Theme {
	te.mu.RLock()
	defer te.mu.RUnlock()
	var list []*Theme
	for _, t := range te.themes {
		list = append(list, t)
	}
	return list
}

// Colorize helpers
func (te *ThemeEngine) P(text string) string {
	return te.Active().Primary + text + Reset
}

func (te *ThemeEngine) S(text string) string {
	return te.Active().Secondary + text + Reset
}

func (te *ThemeEngine) A(text string) string {
	return te.Active().Accent + text + Reset
}

func (te *ThemeEngine) Success(text string) string {
	return te.Active().Success + text + Reset
}

func (te *ThemeEngine) Warning(text string) string {
	return te.Active().Warning + text + Reset
}

func (te *ThemeEngine) Error(text string) string {
	return te.Active().Error + text + Reset
}

func (te *ThemeEngine) Muted(text string) string {
	return te.Active().Muted + text + Reset
}

func (te *ThemeEngine) PrintPalettePreview() {
	t := te.Active()
	fmt.Printf("\n🎨 Active Theme: %s\n", t.DisplayName)
	fmt.Printf("  • Primary   : %s██████████%s (%s)\n", t.Primary, Reset, t.Name)
	fmt.Printf("  • Secondary : %s██████████%s\n", t.Secondary, Reset)
	fmt.Printf("  • Accent    : %s██████████%s\n", t.Accent, Reset)
	fmt.Printf("  • Success   : %s██████████%s\n", t.Success, Reset)
	fmt.Printf("  • Warning   : %s██████████%s\n", t.Warning, Reset)
	fmt.Printf("  • Error     : %s██████████%s\n", t.Error, Reset)
	fmt.Printf("  • Muted     : %s██████████%s\n\n", t.Muted, Reset)
}
