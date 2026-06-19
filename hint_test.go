package main

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestHintRow(t *testing.T) {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
	lab := lipgloss.NewStyle().Bold(true)
	got := hintRow("     ", map[int]string{2: "a"}, 6, dim, lab)
	if strip(got) != "  a   " {
		t.Fatalf("strip = %q, want %q", strip(got), "  a   ")
	}
}
