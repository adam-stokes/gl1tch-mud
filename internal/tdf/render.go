package tdf

import (
	"fmt"
	"strings"
)

var (
	fgANSI = [16]uint8{30, 34, 32, 36, 31, 35, 33, 37, 90, 94, 92, 96, 91, 95, 93, 97}
	bgANSI = [8]uint8{40, 44, 42, 46, 41, 45, 43, 47}
)

func colorEscape(color byte) string {
	fg := color & 0x0f
	bg := (color & 0xf0) >> 4
	if bg >= 8 {
		bg = 0
	}
	return fmt.Sprintf("\x1b[%d;%dm", fgANSI[fg], bgANSI[bg])
}

// RenderString renders text using font f, returning one ANSI-colored string per row.
func RenderString(text string, f *Font) []string {
	// Collect glyphs for each rune in text (skip missing).
	type entry struct {
		g   *Glyph
		sep int // spacing columns after this glyph
	}
	var glyphs []entry
	runes := []rune(text)
	for i, r := range runes {
		g, ok := f.Glyph(r)
		if !ok {
			continue
		}
		sep := 0
		if i < len(runes)-1 {
			sep = int(f.Spacing)
		}
		glyphs = append(glyphs, entry{g, sep})
	}

	lines := make([]string, f.Height)
	for row := range int(f.Height) {
		var sb strings.Builder
		for _, e := range glyphs {
			g := e.g
			var lastColor byte = 255 // invalid sentinel
			for col := range int(g.Width) {
				idx := row*int(g.Width) + col
				if idx >= len(g.Cells) {
					sb.WriteRune(' ')
					continue
				}
				cell := g.Cells[idx]
				if cell.Color != lastColor {
					sb.WriteString(colorEscape(cell.Color))
					lastColor = cell.Color
				}
				sb.WriteRune(cell.Ch)
			}
			sb.WriteString("\x1b[0m")
			// spacing
			for range e.sep {
				sb.WriteRune(' ')
			}
		}
		lines[row] = sb.String()
	}
	return lines
}

// RenderStringThemed renders text using font f with theme-mapped colors.
// Bright TDF foreground indices (8–15) map to accentSeq; dim indices (1–7) map
// to dimSeq; index 0 (black) resets to default. Both sequences should be
// pre-formatted ANSI 24-bit escape strings, e.g. "\x1b[38;2;189;147;249m".
func RenderStringThemed(text string, f *Font, accentSeq, dimSeq string) []string {
	type entry struct {
		g   *Glyph
		sep int
	}
	var glyphs []entry
	runes := []rune(text)
	for i, r := range runes {
		g, ok := f.Glyph(r)
		if !ok {
			continue
		}
		sep := 0
		if i < len(runes)-1 {
			sep = int(f.Spacing)
		}
		glyphs = append(glyphs, entry{g, sep})
	}

	lines := make([]string, f.Height)
	for row := range int(f.Height) {
		var sb strings.Builder
		lastSeq := ""
		for _, e := range glyphs {
			g := e.g
			for col := range int(g.Width) {
				idx := row*int(g.Width) + col
				if idx >= len(g.Cells) {
					sb.WriteRune(' ')
					continue
				}
				cell := g.Cells[idx]
				fg := cell.Color & 0x0f
				var seq string
				switch {
				case fg >= 8:
					seq = accentSeq
				case fg > 0:
					seq = dimSeq
				default:
					seq = "\x1b[0m"
				}
				if seq != lastSeq {
					sb.WriteString(seq)
					lastSeq = seq
				}
				sb.WriteRune(cell.Ch)
			}
			sb.WriteString("\x1b[0m")
			lastSeq = "\x1b[0m"
			for range e.sep {
				sb.WriteRune(' ')
			}
		}
		lines[row] = sb.String()
	}
	return lines
}

// StripANSI removes ANSI escape sequences from s. Exported for tests.
func StripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// skip until 'm' or end
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
