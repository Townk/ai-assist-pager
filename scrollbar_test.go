package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestVThumb(t *testing.T) {
	if p, s := vthumb(10, 20, 0); p != 0 || s != 0 {
		t.Fatalf("fits → (0,0), got (%d,%d)", p, s)
	}
	if _, s := vthumb(100, 10, 0); s < 1 {
		t.Fatalf("size must be ≥1, got %d", s)
	}
	if p, _ := vthumb(100, 10, 0); p != 0 {
		t.Fatalf("top → pos 0, got %d", p)
	}
	p, s := vthumb(100, 10, 90) // maxOff = 90
	if p != 10-s {
		t.Fatalf("bottom → pos == visible-size (%d), got %d", 10-s, p)
	}
}

func TestHThumb(t *testing.T) {
	if p, s := hthumb(40, 80, 5); p != 0 || s != 80 {
		t.Fatalf("blockW≤view → full track (0,view), got (%d,%d)", p, s)
	}
	if p, _ := hthumb(200, 80, 0); p != 0 {
		t.Fatalf("xoff 0 → pos 0, got %d", p)
	}
	p, s := hthumb(200, 80, 120) // maxX = 120
	if p != 80-s {
		t.Fatalf("max xoff → pos == view-size (%d), got %d", 80-s, p)
	}
	if p, _ := hthumb(200, 80, 9999); p != 80-s2(200, 80) {
		t.Fatalf("xoff clamps to maxX")
	}
}

func s2(b, v int) int { _, s := hthumb(b, v, 1<<30); return s }

func TestHScrollbarRowWidthAndGlyphs(t *testing.T) {
	row := hscrollbarRow(200, 0, 40)
	if lipgloss.Width(row) != 40 {
		t.Fatalf("width = %d, want 40", lipgloss.Width(row))
	}
	plain := strip(row)
	for _, r := range plain {
		if r != '─' && r != '━' {
			t.Fatalf("unexpected glyph %q", r)
		}
	}
	_, size := hthumb(200, 40, 0)
	if strings.Count(plain, "━") != size {
		t.Fatalf("thumb run = %d, want %d", strings.Count(plain, "━"), size)
	}
}

func TestPadTo(t *testing.T) {
	if got := padTo("ab", 5); lipgloss.Width(got) != 5 || strip(got) != "ab   " {
		t.Fatalf("padTo = %q", strip(got))
	}
	if got := padTo("abcdef", 3); strip(got) != "abcdef" {
		t.Fatalf("padTo must not truncate, got %q", strip(got))
	}
}

func TestHintCodeRow(t *testing.T) {
	row := "\x1b[38;2;1;2;3mhi\x1b[0m"
	got := hintCodeRow(row, 6)
	if lipgloss.Width(got) != 6 {
		t.Fatalf("width = %d, want 6", lipgloss.Width(got))
	}
	if strip(got) != "hi    " {
		t.Fatalf("strip = %q, want %q", strip(got), "hi    ")
	}
	if !strings.Contains(got, codeBgANSI) {
		t.Fatal("must paint the code-bg fill")
	}
}

func TestOverlayLabels(t *testing.T) {
	lab := lipgloss.NewStyle().Bold(true)
	row := "abcde"
	got := overlayLabels(row, map[int]string{2: "X"}, lab)
	if strip(got) != "abXde" {
		t.Fatalf("strip = %q, want abXde", strip(got))
	}
	if overlayLabels(row, nil, lab) != row {
		t.Fatal("no labels → unchanged")
	}
}
