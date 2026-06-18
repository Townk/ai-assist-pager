package main

import (
	"github.com/alecthomas/chroma/v2"
)

// Catppuccin Mocha.
const (
	colMauve    = "#cba6f7"
	colText     = "#cdd6f4"
	colBase     = "#1e1e2e"
	colMantle   = "#181825" // quote background band
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
