package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func joinText(lines []Line) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = strip(l.Text)
	}
	return strings.Join(parts, "\n")
}

func TestRenderHeadingHasBlockPrefix(t *testing.T) {
	lines, _ := Render("# Title", 40)
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
	lines, _ := Render(md, 20)
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
	lines, _ := Render("- one\n- two", 40)
	got := joinText(lines)
	if !strings.Contains(got, "one") || !strings.Contains(got, "two") {
		t.Fatalf("list items missing:\n%s", got)
	}
	if !strings.Contains(got, "• one") {
		t.Fatalf("bullet marker missing for first item:\n%s", got)
	}
}

func TestRenderOrderedList(t *testing.T) {
	lines, _ := Render("1. first\n2. second", 40)
	got := joinText(lines)
	if !strings.Contains(got, "1. first") {
		t.Fatalf("ordered list item 1 missing:\n%s", got)
	}
	if !strings.Contains(got, "2. second") {
		t.Fatalf("ordered list item 2 missing:\n%s", got)
	}
}

func TestRenderNestedList(t *testing.T) {
	lines, _ := Render("- a\n    - b", 40)
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
	lines, _ := Render("a **bold** word", 40)
	got := joinText(lines)
	if !strings.Contains(got, "bold") {
		t.Fatalf("strong text missing:\n%s", got)
	}
}

func TestRenderCodeBlockIsWideAndUnwrapped(t *testing.T) {
	long := "x := aaaaaaaaaa + bbbbbbbbbb + cccccccccc + dddddddddd // long line"
	md := "```go\n" + long + "\n```"
	lines, _ := Render(md, 20) // pane narrower than the code line
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
	lines, _ := Render("> hello quote", 40)
	got := joinText(lines)
	if !strings.Contains(got, "hello quote") {
		t.Fatalf("quote text missing:\n%s", got)
	}
	if !strings.Contains(got, "▋") {
		t.Fatalf("quote border glyph missing:\n%s", got)
	}
}

func TestRenderQuoteDefaultAdmonition(t *testing.T) {
	lines, _ := Render("> hello there friend", 40)
	// bare quote: NO header line — the first emitted line is a body line.
	first := strip(lines[0].Text)
	if !strings.HasPrefix(first, "▋ ") {
		t.Fatalf("first line should be a body line starting with '▋ ', got: %q", first)
	}
	if !strings.Contains(first, "hello") {
		t.Fatalf("first (body) line should contain the body text, got: %q", first)
	}
	// no line should contain the word "Quote" (no title header)
	for _, l := range lines {
		if strings.Contains(strip(l.Text), "Quote") {
			t.Fatalf("bare quote should have no 'Quote' title header, but found it in: %q", strip(l.Text))
		}
		if l.Wide {
			t.Fatalf("quote line should not be Wide")
		}
	}
}

func TestRenderQuoteAdmonitionType(t *testing.T) {
	lines, _ := Render("> [!note]\n> be careful here", 40)
	hdr := strip(lines[0].Text)
	if !strings.Contains(hdr, "Note") {
		t.Fatalf("expected Note header, got %q", hdr)
	}
	body := joinText(lines)
	if strings.Contains(body, "[!note]") {
		t.Fatalf("admonition marker leaked into the body:\n%s", body)
	}
	if !strings.Contains(body, "be careful here") {
		t.Fatalf("body text missing:\n%s", body)
	}
}

func TestRenderQuoteExplicitQuoteType(t *testing.T) {
	lines, _ := Render("> [!quote]\n> some quoted body", 40)
	// first line is the header: must contain "Quote" and the 󱆨 glyph
	hdr := strip(lines[0].Text)
	if !strings.Contains(hdr, "Quote") {
		t.Fatalf("[!quote] header missing 'Quote' title: %q", hdr)
	}
	if !strings.Contains(hdr, "󱆨") {
		t.Fatalf("[!quote] header missing 󱆨 icon: %q", hdr)
	}
	// body present
	body := joinText(lines)
	if !strings.Contains(body, "some quoted body") {
		t.Fatalf("[!quote] body text missing:\n%s", body)
	}
}

func TestDarken(t *testing.T) {
	got := darken("#FFFFFF", 0.20)
	if got != "#333333" {
		t.Fatalf("darken(#FFFFFF, 0.20) = %q, want #333333", got)
	}
	// darken #89b4fa by 0.20 — all components must be less than original
	origR, origG, origB := parseHex("#89b4fa")
	dr, dg, db := parseHex(darken("#89b4fa", 0.20))
	if dr >= origR || dg >= origG || db >= origB {
		t.Fatalf("darken(#89b4fa, 0.20) = %q; expected all components < originals (%d,%d,%d)", darken("#89b4fa", 0.20), origR, origG, origB)
	}
}

func TestBandFillsWidthWithBg(t *testing.T) {
	bg := "\x1b[48;2;1;1;1m"
	result := band("x", bg, 10)
	if !strings.HasPrefix(result, bg) {
		t.Fatalf("band result does not start with bg sequence: %q", result)
	}
	if !strings.HasSuffix(result, "\x1b[0m") {
		t.Fatalf("band result does not end with reset: %q", result)
	}
	if w := lipgloss.Width(result); w != 10 {
		t.Fatalf("band width = %d, want 10", w)
	}
}

func TestRenderQuoteReflowsToWidth(t *testing.T) {
	long := "> " + strings.Repeat("word ", 40)
	reflowed, _ := Render(long, 30)
	for _, l := range reflowed {
		if w := lipgloss.Width(l.Text); w > 30 {
			t.Fatalf("quote line exceeds content width 30 (got %d): %q", w, strip(l.Text))
		}
	}
}

func TestRenderCodeBlockNamedLanguageHighlights(t *testing.T) {
	md := "```go\npackage main\n```"
	lines, _ := Render(md, 80)
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
	lines, _ := Render(md, 80)
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

func TestRenderTableIsWide(t *testing.T) {
	md := "| Col A | Col B |\n|---|---|\n| one | two |\n| three | four |"
	lines, _ := Render(md, 12)
	wide := false
	for _, l := range lines {
		if l.Wide {
			wide = true
		}
	}
	if !wide {
		t.Fatalf("table produced no Wide lines:\n%s", joinText(lines))
	}
	if !strings.Contains(joinText(lines), "Col A") || !strings.Contains(joinText(lines), "four") {
		t.Fatalf("table cells missing:\n%s", joinText(lines))
	}
	if strings.Contains(joinText(lines), "---") {
		t.Fatalf("table separator row leaked into output:\n%s", joinText(lines))
	}
}

func TestStripRemovesFullSGR(t *testing.T) {
	in := "a\x1b[1mbold\x1b[0m b\x1b[38;2;1;2;3mc\x1b[0m"
	if got := strip(in); got != "abold bc" {
		t.Fatalf("strip = %q, want %q", got, "abold bc")
	}
}

func TestRenderCodeBlockBackgroundStretchesAndSurvives(t *testing.T) {
	lines, _ := Render("```go\nx := 1\n```", 40)
	var code *Line
	for i := range lines {
		if lines[i].Wide {
			code = &lines[i]
			break
		}
	}
	if code == nil {
		t.Fatal("no wide code line")
	}
	// bg is now carried in the Bg field, not baked into Text
	if code.Bg != codeBgANSI {
		t.Fatalf("code line Bg = %q, want codeBgANSI", code.Bg)
	}
	// Text must NOT already contain a background sequence (it's fg-only)
	if strings.Contains(code.Text, "48;2") {
		t.Fatalf("code line Text contains a background sequence; it should be fg-only: %q", code.Text)
	}
	// render through the viewport: backdrop must fill the full viewport width
	out := Window([]Line{*code}, 0, 0, 40, 1)[0]
	if !strings.HasPrefix(out, codeBgANSI) {
		t.Fatalf("viewport output does not open with bg sequence")
	}
	if w := lipgloss.Width(out); w != 40 {
		t.Fatalf("viewport output width = %d, want 40", w)
	}
	// every "\x1b[0m" inside (except the trailing one) must be followed by the bg re-apply
	inner := strings.TrimSuffix(out, "\x1b[0m")
	if strings.Contains(inner, "\x1b[0m") && !strings.Contains(inner, "\x1b[0m"+codeBgANSI) {
		t.Fatalf("a reset is not followed by a bg re-apply (bg would drop): %q", out)
	}
}

func TestRenderCodeBlockLanguageLabel(t *testing.T) {
	// Use a language NOT in the icon map so we exercise the text-fallback path.
	lines, _ := Render("```text\nx := 1\n```", 40)
	// first line is the top-label bar, Wide=false.
	if lines[0].Wide {
		t.Fatal("label line should not be Wide")
	}
	got := strip(lines[0].Text)
	// The language name must appear in the label region.
	if !strings.Contains(got, "text") {
		t.Fatalf("label line %q should contain the language name %q", got, "text")
	}
	// Left fill contains the ▂ bar character.
	if !strings.Contains(got, "▂") {
		t.Fatalf("label line %q should contain '▂' fill", got)
	}
	// Total display width must be exactly the content width (40).
	if w := lipgloss.Width(lines[0].Text); w != 40 {
		t.Fatalf("label line display width = %d, want 40", w)
	}
}

func TestRenderCodeBlockIconLabel(t *testing.T) {
	// Use 'go' which has an icon — verify the tab shows the glyph and ▂ fill.
	lines, _ := Render("```go\nx := 1\n```", 40)
	if lines[0].Wide {
		t.Fatal("icon label line should not be Wide")
	}
	got := strip(lines[0].Text)
	// The icon glyph for go must appear in the label.
	goGlyph := langIcons["go"].glyph
	if !strings.Contains(got, goGlyph) {
		t.Fatalf("icon label line %q should contain go glyph %q", got, goGlyph)
	}
	// Left fill still has ▂.
	if !strings.Contains(got, "▂") {
		t.Fatalf("icon label line %q should contain '▂' fill", got)
	}
	// Total display width must be exactly the content width (40).
	if w := lipgloss.Width(lines[0].Text); w != 40 {
		t.Fatalf("icon label line display width = %d, want 40", w)
	}
}

func TestRenderCodeTabIconAndLabel(t *testing.T) {
	lines, _ := Render("```rust\nx\n```", 60)
	var tab string
	for _, l := range lines {
		if strings.Contains(strip(l.Text), "❘") { // the tab line has the separator
			tab = strip(l.Text)
			break
		}
	}
	if tab == "" {
		t.Fatal("no tab line found")
	}
	if !strings.Contains(tab, "rust") {
		t.Fatalf("tab should show the language label; got %q", tab)
	}
	rustGlyph := langIcons["rust"].glyph
	if !strings.Contains(tab, rustGlyph) {
		t.Fatalf("tab should still show the rust icon glyph; got %q", tab)
	}
}

func TestRenderCodeBlockBottomBar(t *testing.T) {
	lines, _ := Render("```go\nx := 1\n```", 40)
	// last line is the bottom edge bar, Wide=false, filled with 🮂, width == 40.
	last := lines[len(lines)-1]
	if last.Wide {
		t.Fatal("bottom bar line should not be Wide")
	}
	got := strip(last.Text)
	if !strings.Contains(got, "🮂") {
		t.Fatalf("bottom bar line %q should contain '🮂'", got)
	}
	if w := lipgloss.Width(last.Text); w != 40 {
		t.Fatalf("bottom bar line display width = %d, want 40", w)
	}
}

func TestIsShellLang(t *testing.T) {
	for _, s := range []string{"sh", "bash", "zsh", "console", "shell", "shell-session"} {
		if !isShellLang(s) {
			t.Fatalf("isShellLang(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"go", "python", ""} {
		if isShellLang(s) {
			t.Fatalf("isShellLang(%q) = true, want false", s)
		}
	}
}

func TestCodeBlockButtonsShell(t *testing.T) {
	_, btns := Render("```sh\nmake all\n```", 40)
	var play, copyB *Button
	for i := range btns {
		switch btns[i].Kind {
		case "play":
			play = &btns[i]
		case "copy":
			copyB = &btns[i]
		}
	}
	if play == nil || copyB == nil {
		t.Fatalf("shell block must yield play+copy, got %+v", btns)
	}
	if play.Payload != "make all" || copyB.Payload != "make all" {
		t.Fatalf("payload = %q/%q, want raw source", play.Payload, copyB.Payload)
	}
	if play.Width != 2 || copyB.Width != 2 {
		t.Fatalf("button width must be 2 (glyph+trailing space)")
	}
	if !(play.Col < copyB.Col) {
		t.Fatalf("play must be left of copy: %d vs %d", play.Col, copyB.Col)
	}
	if play.Line != copyB.Line {
		t.Fatalf("both buttons live on the same tab line")
	}
}

func TestCodeBlockButtonsNonShell(t *testing.T) {
	_, btns := Render("```python\nx=1\n```", 40)
	kinds := map[string]int{}
	for _, b := range btns {
		kinds[b.Kind]++
	}
	if kinds["copy"] != 1 || kinds["play"] != 0 {
		t.Fatalf("non-shell block: want exactly 1 copy + 0 play, got %v", kinds)
	}
}

func TestCodeBlockLineMarkers(t *testing.T) {
	// Narrow content: no horizontal overflow → no HBar row.
	lines, _ := Render("```go\nx := 1\n```", 40)
	codeCount, hbar := 0, 0
	for _, l := range lines {
		if l.Code {
			codeCount++
		}
		if l.HBar > 0 {
			hbar++
		}
	}
	if codeCount < 3 { // tab + ≥1 body + bottom bar
		t.Fatalf("expected ≥3 Code-tagged lines, got %d", codeCount)
	}
	if hbar != 0 {
		t.Fatalf("narrow block must not emit an HBar row, got %d", hbar)
	}
}

func TestCodeBlockHBarOnOverflow(t *testing.T) {
	long := "xy " + strings.Repeat("z", 200)
	lines, _ := Render("```go\n"+long+"\n```", 40)
	hbarIdx := -1
	for i, l := range lines {
		if l.HBar > 0 {
			if hbarIdx != -1 {
				t.Fatal("expected exactly one HBar row")
			}
			hbarIdx = i
			if !l.Code {
				t.Fatal("HBar row must be Code-tagged")
			}
			if l.HBar <= 40 {
				t.Fatalf("HBar width should be the block width (>40), got %d", l.HBar)
			}
		}
	}
	if hbarIdx == -1 {
		t.Fatal("overflowing block must emit an HBar row")
	}
	// It sits between the last body line and the bottom bar (the next line is the bar).
	if hbarIdx+1 >= len(lines) || lines[hbarIdx].Wide {
		t.Fatal("HBar row should be a non-Wide row before the bottom bar")
	}
}
