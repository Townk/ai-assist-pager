package main

import (
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
)

// Catppuccin Mocha.
const (
	colMauve    = "#cba6f7"
	colText     = "#cdd6f4"
	colBase     = "#1e1e2e"
	colMantle   = "#181825" // code / quote background band
	colOverlay0 = "#6c7086"
	colOverlay1 = "#7f849c"
	colSurface0 = "#313244"
	colBlue     = "#89b4fa"
	colGreen    = "#a6e3a1"
	colPeach    = "#fab387"
	colYellow   = "#f9e2af"
	colLavender = "#b4befe"
	colSky      = "#89dceb"
	colSubtext  = "#9399b2"
)

// bandStyle paints a full-width background band (code blocks, block quotes).
func bandStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colText)).
		Background(lipgloss.Color(colMantle))
}

// codeStyle is a chroma style built from the Catppuccin token colors (the same
// map the glow theme uses), so code highlighting matches the rest of the UI
// regardless of whether the chroma version ships a Catppuccin style.
var catppuccinChroma = chroma.MustNewStyle("catppuccin-mocha", chroma.StyleEntries{
	chroma.Background:     "bg:" + colMantle,
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
