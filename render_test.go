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
	if !strings.Contains(got, "• one") {
		t.Fatalf("bullet marker missing for first item:\n%s", got)
	}
}

func TestRenderOrderedList(t *testing.T) {
	got := joinText(Render("1. first\n2. second", 40))
	if !strings.Contains(got, "1. first") {
		t.Fatalf("ordered list item 1 missing:\n%s", got)
	}
	if !strings.Contains(got, "2. second") {
		t.Fatalf("ordered list item 2 missing:\n%s", got)
	}
}

func TestRenderNestedList(t *testing.T) {
	lines := Render("- a\n    - b", 40)
	got := joinText(lines)
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Fatalf("nested list items missing:\n%s", got)
	}
	// Find the indentation of the line containing "a" and "b" and assert that
	// the nested item "b" is more indented than the parent item "a".
	indentOf := func(needle string) int {
		for _, l := range lines {
			plain := strip(l.Text)
			if strings.Contains(plain, needle) {
				return len(plain) - len(strings.TrimLeft(plain, " "))
			}
		}
		return -1
	}
	indentA := indentOf("a")
	indentB := indentOf("b")
	if indentA < 0 || indentB < 0 {
		t.Fatalf("could not locate 'a' or 'b' in output:\n%s", got)
	}
	if indentB <= indentA {
		t.Fatalf("nested item 'b' (indent %d) is not more indented than 'a' (indent %d):\n%s", indentB, indentA, got)
	}
}

func TestRenderInlineStrongText(t *testing.T) {
	// The bold word's text survives (styling is stripped in the assertion).
	got := joinText(Render("a **bold** word", 40))
	if !strings.Contains(got, "bold") {
		t.Fatalf("strong text missing:\n%s", got)
	}
}

func TestRenderCodeBlockIsWideAndUnwrapped(t *testing.T) {
	long := "x := aaaaaaaaaa + bbbbbbbbbb + cccccccccc + dddddddddd // long line"
	md := "```go\n" + long + "\n```"
	lines := Render(md, 20) // pane narrower than the code line
	var codeLine *Line
	for i := range lines {
		if lines[i].Wide {
			codeLine = &lines[i]
			break
		}
	}
	if codeLine == nil {
		t.Fatalf("expected a Wide code line, got none:\n%s", joinText(lines))
	}
	if w := len(strip(codeLine.Text)); w <= 20 {
		t.Fatalf("code line was wrapped/truncated to width (len=%d); it must keep natural width", w)
	}
	if !strings.Contains(codeLine.Text, "\x1b[") {
		t.Fatalf("code line is not styled (no ANSI): %q", codeLine.Text)
	}
}

func TestRenderBlockQuote(t *testing.T) {
	got := joinText(Render("> hello quote", 40))
	if !strings.Contains(got, "hello quote") {
		t.Fatalf("quote text missing:\n%s", got)
	}
	if !strings.Contains(got, "│") {
		t.Fatalf("quote indent token missing:\n%s", got)
	}
}

func TestRenderCodeBlockNamedLanguageHighlights(t *testing.T) {
	md := "```go\npackage main\n```"
	lines := Render(md, 80)
	var codeLine *Line
	for i := range lines {
		if lines[i].Wide {
			codeLine = &lines[i]
			break
		}
	}
	if codeLine == nil {
		t.Fatalf("expected a Wide code line, got none:\n%s", joinText(lines))
	}
	if !strings.Contains(codeLine.Text, "\x1b[") {
		t.Fatalf("named language 'go' was not highlighted (no ANSI escape in output): %q", codeLine.Text)
	}
}

func TestRenderCodeBlockUnknownLanguageNoPanic(t *testing.T) {
	md := "```unknown_xyz\nhello world\n```"
	lines := Render(md, 80)
	var codeLine *Line
	for i := range lines {
		if lines[i].Wide {
			codeLine = &lines[i]
			break
		}
	}
	if codeLine == nil {
		t.Fatalf("expected a Wide code line, got none:\n%s", joinText(lines))
	}
	if !strings.Contains(strip(codeLine.Text), "hello world") {
		t.Fatalf("code text missing from unknown-language block:\n%s", joinText(lines))
	}
}
