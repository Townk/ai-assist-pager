package main

import (
	"strings"
	"testing"
)

func joinText(lines []Line) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = strip(l.Text)
	}
	return strings.Join(parts, "\n")
}

func TestRenderHeadingHasBlockPrefix(t *testing.T) {
	lines := Render("# Title", 40)
	if !strings.Contains(joinText(lines), "▓▓▓ Title") {
		t.Fatalf("heading missing ▓▓▓ prefix:\n%s", joinText(lines))
	}
	for _, l := range lines {
		if l.Wide {
			t.Fatalf("heading line should not be Wide")
		}
	}
}

func TestRenderParagraphWraps(t *testing.T) {
	md := "alpha beta gamma delta epsilon zeta eta theta iota kappa"
	lines := Render(md, 20)
	for _, l := range lines {
		if l.Wide {
			t.Fatalf("paragraph line should not be Wide")
		}
		if w := len(strip(l.Text)); w > 20 {
			t.Fatalf("paragraph line %q exceeds width 20 (got %d)", strip(l.Text), w)
		}
	}
	if len(lines) < 2 {
		t.Fatalf("expected the paragraph to wrap to multiple lines, got %d", len(lines))
	}
}

func TestRenderListItems(t *testing.T) {
	got := joinText(Render("- one\n- two", 40))
	if !strings.Contains(got, "one") || !strings.Contains(got, "two") {
		t.Fatalf("list items missing:\n%s", got)
	}
}

func TestRenderInlineStrongText(t *testing.T) {
	// The bold word's text survives (styling is stripped in the assertion).
	got := joinText(Render("a **bold** word", 40))
	if !strings.Contains(got, "bold") {
		t.Fatalf("strong text missing:\n%s", got)
	}
}
