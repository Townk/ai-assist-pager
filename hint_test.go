package main

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestHintRow(t *testing.T) {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
	lab := lipgloss.NewStyle().Bold(true)
	got := hintRow("     ", map[int]string{2: "a"}, nil, 6, dim, lab)
	if strip(got) != "  a   " {
		t.Fatalf("strip = %q, want %q", strip(got), "  a   ")
	}
}

func TestHintRowKeepStylesWithoutMovingChars(t *testing.T) {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
	lab := lipgloss.NewStyle().Bold(true)
	withKeep := hintRow("xyz", nil, map[int]string{1: colGreen}, 5, dim, lab)
	without := hintRow("xyz", nil, nil, 5, dim, lab)
	// Keeping a cell changes only its styling, never the visible layout.
	if strip(withKeep) != strip(without) {
		t.Fatalf("keep changed layout: %q vs %q", strip(withKeep), strip(without))
	}
	if withKeep == without {
		t.Fatal("keep should restyle the kept cell (output bytes must differ from the all-dim row)")
	}
}
