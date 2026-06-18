package main

import (
	"fmt"
	"strconv"

	"github.com/alecthomas/chroma/v2"
)

// parseHex returns the r,g,b components of a "#RRGGBB" hex string.
func parseHex(hex string) (int, int, int) {
	h := hex
	if len(h) > 0 && h[0] == '#' {
		h = h[1:]
	}
	rv, _ := strconv.ParseInt(h[0:2], 16, 32)
	gv, _ := strconv.ParseInt(h[2:4], 16, 32)
	bv, _ := strconv.ParseInt(h[4:6], 16, 32)
	return int(rv), int(gv), int(bv)
}

// darken scales a #RRGGBB color toward black by factor f (0..1); returns "#RRGGBB".
// Use f≈0.20 for a "very dark" tint.
func darken(hex string, f float64) string {
	rv, gv, bv := parseHex(hex)
	return fmt.Sprintf("#%02X%02X%02X", int(float64(rv)*f), int(float64(gv)*f), int(float64(bv)*f))
}

// bgANSI returns the truecolor background SGR sequence for a #RRGGBB hex color.
func bgANSI(hex string) string {
	rv, gv, bv := parseHex(hex)
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", rv, gv, bv)
}

// Catppuccin Mocha.
const (
	colMauve    = "#cba6f7"
	colText     = "#cdd6f4"
	colBase     = "#1e1e2e"
	colCodeBg   = "#282C41" // code block background
	colOverlay0 = "#6c7086"
	colOverlay1 = "#7f849c"
	colSurface0 = "#313244"
	colBlue     = "#89b4fa"
	colGreen    = "#a6e3a1"
	colPeach    = "#fab387"
	colRed      = "#f38ba8"
	colYellow   = "#f9e2af"
	colLavender = "#b4befe"
	colSky      = "#89dceb"
	colSubtext  = "#9399b2"
)

// codeBgANSI is the code block background (#282C41 = R40 G44 B65) applied
// manually so it survives chroma's per-token resets.
const codeBgANSI = "\x1b[48;2;40;44;65m"

// codeFgANSI is the foreground-only version of colCodeBg (#282C41 = R40 G44 B65),
// used to draw the top/bottom edge bars with no background.
const codeFgANSI = "\x1b[38;2;40;44;65m"

// codeStyle is a chroma style built from the Catppuccin token colors (the same
// map the glow theme uses), so code highlighting matches the rest of the UI
// regardless of whether the chroma version ships a Catppuccin style.
var catppuccinChroma = chroma.MustNewStyle("catppuccin-mocha", chroma.StyleEntries{
	chroma.Text:           colText,
	chroma.Comment:        colOverlay0,
	chroma.CommentPreproc: colBlue,
	chroma.Keyword:        colMauve,
	chroma.KeywordType:    colYellow,
	chroma.Operator:       colSky,
	chroma.Punctuation:    colSubtext,
	chroma.Name:           colLavender,
	chroma.NameBuiltin:    colPeach,
	chroma.NameFunction:   colBlue,
	chroma.NameClass:      colYellow,
	chroma.NameTag:        colMauve,
	chroma.NameAttribute:  colYellow,
	chroma.LiteralNumber:  colPeach,
	chroma.LiteralString:  colGreen,
})

func codeStyle() *chroma.Style { return catppuccinChroma }
