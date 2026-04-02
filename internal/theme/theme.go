// Package theme loads the active gl1tch colour palette and exposes ANSI helpers.
package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Palette holds the active colour palette.
type Palette struct {
	BG      string `yaml:"bg"`
	FG      string `yaml:"fg"`
	Accent  string `yaml:"accent"`
	Dim     string `yaml:"dim"`
	Border  string `yaml:"border"`
	Error   string `yaml:"error"`
	Success string `yaml:"success"`
}

// dracula is the fallback palette used when no theme file is found.
var dracula = &Palette{
	Accent:  "#bd93f9",
	Dim:     "#6272a4",
	FG:      "#f8f8f2",
	BG:      "#282a36",
	Border:  "#6272a4",
	Error:   "#ff5555",
	Success: "#50fa7b",
}

// Load reads the active theme from ~/.config/glitch/active_theme and loads
// the corresponding palette from ~/.config/glitch/themes/<name>/theme.yaml.
// Falls back to the Dracula palette on any error.
func Load() *Palette {
	home, err := os.UserHomeDir()
	if err != nil {
		return dracula
	}

	activeFile := filepath.Join(home, ".config", "glitch", "active_theme")
	raw, err := os.ReadFile(activeFile)
	if err != nil {
		return dracula
	}
	name := strings.TrimSpace(string(raw))
	if name == "" {
		return dracula
	}

	themeFile := filepath.Join(home, ".config", "glitch", "themes", name, "theme.yaml")
	data, err := os.ReadFile(themeFile)
	if err != nil {
		return dracula
	}

	var p Palette
	if err := yaml.Unmarshal(data, &p); err != nil {
		return dracula
	}
	return &p
}

// FgSeq converts a hex colour string (e.g. "#bd93f9") to an ANSI 24-bit
// foreground escape sequence: \x1b[38;2;R;G;Bm.
func FgSeq(hex string) string {
	r, g, b, err := parseHex(hex)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

// parseHex parses a "#rrggbb" string into r, g, b components.
func parseHex(hex string) (r, g, b uint8, err error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex colour: %q", hex)
	}
	rv, e := strconv.ParseUint(hex[0:2], 16, 8)
	if e != nil {
		return 0, 0, 0, e
	}
	gv, e := strconv.ParseUint(hex[2:4], 16, 8)
	if e != nil {
		return 0, 0, 0, e
	}
	bv, e := strconv.ParseUint(hex[4:6], 16, 8)
	if e != nil {
		return 0, 0, 0, e
	}
	return uint8(rv), uint8(gv), uint8(bv), nil
}
