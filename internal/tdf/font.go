// Package tdf implements a native Go renderer for TheDraw Font (.tdf) files.
package tdf

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	magic      = "\x13TheDraw FONTS file\x1a"
	magicLen   = 20
	numChars   = 94
	dataOffset = 233
	charsetStr = "!\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"
	colorFont  = 2
)

// Cell holds one rendered column in a glyph row.
type Cell struct {
	Ch    rune
	Color byte // raw TDF color byte: fg=bits0-3, bg=bits4-7
}

// Glyph holds the decoded cell grid for one character.
type Glyph struct {
	Width  uint8
	Height uint8
	Cells  []Cell // row-major: Cells[row*Width + col]
}

// Font holds the parsed TDF font data.
type Font struct {
	Name    string
	Height  uint8
	Spacing uint8
	glyphs  [numChars]*Glyph
}

// HasGlyph returns true if the font contains a glyph for c.
func (f *Font) HasGlyph(c byte) bool {
	_, ok := f.Glyph(rune(c))
	return ok
}

// Glyph returns the glyph for character c (as a rune), if present.
func (f *Font) Glyph(c rune) (*Glyph, bool) {
	idx := strings.IndexRune(charsetStr, c)
	if idx < 0 || idx >= numChars {
		return nil, false
	}
	g := f.glyphs[idx]
	if g == nil {
		return nil, false
	}
	return g, true
}

// ParseFont parses TDF font data from a raw byte slice.
func ParseFont(data []byte) (*Font, error) {
	return Load(data)
}

// Load parses TDF font data from a raw byte slice.
func Load(data []byte) (*Font, error) {
	if len(data) < dataOffset {
		return nil, fmt.Errorf("tdf: file too short")
	}
	if string(data[:magicLen]) != magic {
		return nil, fmt.Errorf("tdf: invalid magic")
	}
	if data[41] != colorFont {
		return nil, fmt.Errorf("tdf: only COLOR fonts supported (type=%d)", data[41])
	}

	nameLen := int(data[24])
	name := string(data[25 : 25+nameLen])

	spacing := data[42]

	// charlist: 94 uint16 LE values at offset 45
	charlist := make([]uint16, numChars)
	for i := range numChars {
		charlist[i] = binary.LittleEndian.Uint16(data[45+i*2 : 47+i*2])
	}

	fontData := data[dataOffset:]

	f := &Font{Name: name, Spacing: spacing}

	for i := range numChars {
		if charlist[i] == 0xffff {
			continue
		}
		offset := int(charlist[i])
		if offset+2 > len(fontData) {
			continue
		}
		g := readGlyph(fontData[offset:])
		if g != nil {
			if g.Height > f.Height {
				f.Height = g.Height
			}
			f.glyphs[i] = g
		}
	}

	// Normalise all glyph cell slices to font height (pad missing rows with spaces).
	for i := range numChars {
		g := f.glyphs[i]
		if g == nil {
			continue
		}
		want := int(g.Width) * int(f.Height)
		if len(g.Cells) < want {
			extra := make([]Cell, want-len(g.Cells))
			for j := range extra {
				extra[j] = Cell{Ch: ' '}
			}
			g.Cells = append(g.Cells, extra...)
		}
	}

	return f, nil
}

func readGlyph(p []byte) *Glyph {
	if len(p) < 2 {
		return nil
	}
	g := &Glyph{
		Width:  p[0],
		Height: p[1],
	}
	if g.Width == 0 {
		return nil
	}

	cells := make([]Cell, int(g.Width)*int(g.Height))
	for i := range cells {
		cells[i] = Cell{Ch: ' '}
	}

	row, col := 0, 0
	i := 2
	for i < len(p) {
		ch := p[i]
		i++
		if ch == 0 {
			break
		}
		if ch == '\r' {
			row++
			col = 0
			continue
		}
		if i >= len(p) {
			break
		}
		color := p[i]
		i++

		var r rune
		if ch < 0x20 {
			r = ' '
		} else {
			r = CP437ToRune(ch)
		}

		idx := row*int(g.Width) + col
		if idx < len(cells) {
			cells[idx] = Cell{Ch: r, Color: color}
		}
		col++
	}

	g.Cells = cells
	return g
}
