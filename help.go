package main

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type helpBind struct{ keys, desc string }
type helpGroup struct {
	title string
	binds []helpBind
}

var helpGroups = []helpGroup{
	{"Movement", []helpBind{
		{"J / ↓", "down one line"},
		{"K / ↑", "up one line"},
		{"󰘴 D / 󰘴 U", "half page down / up"},
		{"󰘴 F / 󰘴 B", "full page down / up"},
		{"G / 󰘶 G", "top / bottom"},
	}},
	{"Horizontal", []helpBind{
		{"H / L", "left / right one column"},
		{"󰘶 H / 󰘶 L", "left / right half-width"},
		{"0 / $", "line start / end"},
	}},
	{"Actions", []helpBind{
		{"󱁐", "hint mode — activate a button"},
		{"󰳽", "activate a button (mouse)"},
		{"?", "toggle this help"},
		{"q / 󱊷", "quit"},
	}},
}

// buildHelpLines renders the keybinding cheatsheet as Wide lines: a leading
// blank, then per group a bold header + underline and right-aligned, styled
// bindings with a description column, with a blank line between groups.
func buildHelpLines() []Line {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color(colMauve)).Bold(true)
	rule := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colYellow))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colSubtext))

	keyW := 0
	for _, g := range helpGroups {
		for _, b := range g.binds {
			if w := lipgloss.Width(b.keys); w > keyW {
				keyW = w
			}
		}
	}

	var out []Line
	add := func(s string) { out = append(out, Line{Text: s, Wide: true}) }
	add("") // top pad
	for gi, g := range helpGroups {
		if gi > 0 {
			add("")
		}
		add("  " + header.Render(g.title))
		add("  " + rule.Render(strings.Repeat("─", lipgloss.Width(g.title))))
		for _, b := range g.binds {
			pad := keyW - lipgloss.Width(b.keys)
			if pad < 0 {
				pad = 0
			}
			add("  " + strings.Repeat(" ", pad) + keyStyle.Render(b.keys) + "  " + descStyle.Render(b.desc))
		}
	}
	return out
}
