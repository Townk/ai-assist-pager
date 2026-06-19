package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func key(s string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: rune(s[0]), Text: s} }

func TestStatusBarPerMode(t *testing.T) {
	m := newModel("T", "hi")
	if !strings.Contains(m.statusBar(), "?: keys") || !strings.Contains(m.statusBar(), "\U000F1050") {
		t.Fatalf("normal status bar wrong: %q", strip(m.statusBar()))
	}
	cancel := "\U000F12B7"
	hm := m
	hm.hintMode = true
	if !strings.Contains(strip(hm.statusBar()), "cancel") || !strings.Contains(hm.statusBar(), cancel) {
		t.Fatalf("hint status bar wrong: %q", strip(hm.statusBar()))
	}
	pm := m
	pm.helpMode = true
	if pm.statusBar() != hm.statusBar() {
		t.Fatal("help status bar must equal the hint cancel bar")
	}
}

// TestHelpInnerDims tests the method form with content-capped values.
func TestHelpInnerDims(t *testing.T) {
	// Generous pane: content should fit (no cap hit).
	m := newModel("T", "hi")
	m.width, m.height = 120, 40
	w, h := m.helpInnerDims()
	if w < 1 || h < 1 {
		t.Fatalf("generous pane gave non-positive dims: %d x %d", w, h)
	}
	// Content fits → dims equal the content size (not the cap).
	contentW := MaxWideWidth(m.helpLines)
	contentH := len(m.helpLines)
	if w != contentW {
		t.Fatalf("generous pane: innerW %d != contentW %d (should be content-sized)", w, contentW)
	}
	if h != contentH {
		t.Fatalf("generous pane: innerH %d != contentH %d (should be content-sized)", h, contentH)
	}

	// Tiny pane: both dims should be floored at 1.
	mt := newModel("T", "hi")
	mt.width, mt.height = 8, 5
	tw, th := mt.helpInnerDims()
	if tw < 1 || th < 1 {
		t.Fatalf("tiny pane must still give ≥1x1: %d x %d", tw, th)
	}

	// Medium pane where caps kick in: verify capping math.
	mc := newModel("T", "hi")
	mc.width, mc.height = 40, 20
	cw := mc.contentWidth()
	capW := cw - 14
	capH := mc.height - 11
	cw2, ch2 := mc.helpInnerDims()
	if capW > 0 && cw2 > capW {
		t.Fatalf("medium pane: innerW %d exceeds cap %d", cw2, capW)
	}
	if capH > 0 && ch2 > capH {
		t.Fatalf("medium pane: innerH %d exceeds cap %d", ch2, capH)
	}
}

func TestHelpClampScroll(t *testing.T) {
	m := newModel("T", "hi")
	m.width, m.height = 80, 24
	m.helpYOff, m.helpXOff = 9999, 9999
	m.clampHelpScroll()
	if m.helpYOff < 0 || m.helpXOff < 0 {
		t.Fatal("offsets must not go negative")
	}
	_, innerH := m.helpInnerDims()
	if max := len(m.helpLines) - innerH; max >= 0 && m.helpYOff > max {
		t.Fatalf("helpYOff %d exceeds max %d", m.helpYOff, max)
	}
}

func TestHelpModalFitsAllPanes(t *testing.T) {
	for _, d := range [][2]int{{80, 24}, {50, 18}, {30, 12}, {120, 40}, {28, 11}, {24, 9}, {20, 8}} {
		m := newModel("T", "hi")
		m.width, m.height = d[0], d[1]
		m.helpMode = true
		out := m.helpModal()
		if want := m.height - 6; lipgloss.Height(out) != want {
			t.Fatalf("%dx%d: modal height %d != area %d (H-6)", d[0], d[1], lipgloss.Height(out), want)
		}
		for i, line := range strings.Split(out, "\n") {
			if w := lipgloss.Width(line); w != m.contentWidth() {
				t.Fatalf("%dx%d: line %d width %d != cw %d", d[0], d[1], i, w, m.contentWidth())
			}
		}
	}
}

func TestViewHeightAllModes(t *testing.T) {
	for _, d := range [][2]int{{80, 24}, {50, 18}, {120, 40}} {
		base := newModel("T", "hello\nworld")
		base.width, base.height = d[0], d[1]
		base.reflow()
		for _, mode := range []string{"normal", "hint", "help"} {
			m := base
			switch mode {
			case "hint":
				m.hintMode = true
			case "help":
				m.helpMode = true
			}
			got := m.viewString()
			if h := lipgloss.Height(got); h != m.height {
				t.Fatalf("%dx%d %s: View height %d != %d", d[0], d[1], mode, h, m.height)
			}
		}
	}
}

func TestHelpTransitions(t *testing.T) {
	m := newModel("T", "hi")
	m.width, m.height = 80, 24
	// ? opens help, zeroes offsets
	m.helpYOff, m.helpXOff = 3, 3
	m2, _ := m.Update(key("?"))
	hm := m2.(model)
	if !hm.helpMode || hm.helpYOff != 0 || hm.helpXOff != 0 {
		t.Fatalf("? should open help and zero offsets: %+v", hm.helpMode)
	}
	// j scrolls help, not the document
	hm.helpLines = append(hm.helpLines, make([]Line, 200)...) // ensure scrollable
	docY := hm.yOff
	m3, _ := hm.Update(key("j"))
	hm3 := m3.(model)
	if hm3.yOff != docY {
		t.Fatal("help scroll must not move the document")
	}
	// esc / q / ? close help
	for _, k := range []string{"esc", "q", "?"} {
		cm, _ := hm3.Update(tea.KeyPressMsg{Text: k})
		if cm.(model).helpMode {
			t.Fatalf("%q must close help", k)
		}
	}
}

// TestHelpSlashAlignment verifies that " / " appears at the same column in
// every binding that contains it (ANSI stripped).
func TestHelpSlashAlignment(t *testing.T) {
	lines := buildHelpLines()
	sepCol := -1
	for _, l := range lines {
		plain := strip(l.Text)
		// Only check binding lines (they have the 2-col indent and a " / ").
		if !strings.HasPrefix(plain, "  ") {
			continue
		}
		idx := strings.Index(plain, " / ")
		if idx < 0 {
			continue
		}
		// Use rune count so multi-byte chars measure correctly.
		col := utf8.RuneCountInString(plain[:idx])
		if sepCol == -1 {
			sepCol = col
		} else if col != sepCol {
			t.Fatalf("'/ ' not aligned: col %d vs %d in %q", col, sepCol, plain)
		}
	}
	if sepCol == -1 {
		t.Fatal("no ' / ' found in help lines")
	}
}

// TestHelpContentSizeWithinMargins verifies that on a generous pane the box is
// content-sized and that innerW/innerH are within the margin caps.
func TestHelpContentSizeWithinMargins(t *testing.T) {
	m := newModel("T", "hi")
	m.width, m.height = 120, 40
	innerW, innerH := m.helpInnerDims()
	cw := m.contentWidth()

	// Content-sized: equal to the actual content dimensions.
	if want := MaxWideWidth(m.helpLines); innerW != want {
		t.Fatalf("innerW %d != content width %d", innerW, want)
	}
	if want := len(m.helpLines); innerH != want {
		t.Fatalf("innerH %d != content height %d", innerH, want)
	}

	// Within margin caps (cw-14 wide; H-11 tall = modal area H-6 minus chrome 5).
	if innerW > cw-14 {
		t.Fatalf("innerW %d exceeds cap cw(%d)-14 = %d", innerW, cw, cw-14)
	}
	if innerH > m.height-11 {
		t.Fatalf("innerH %d exceeds cap H(%d)-11 = %d", innerH, m.height, m.height-11)
	}
}

// TestHelpModalScrollThreshold pins the rule: the modal area is H-6 and the box
// chrome is 5, so the full content (len(helpLines) rows) fits without scrolling
// exactly when len <= H-11, i.e. at pane height len+11 and taller; one row
// shorter, it scrolls.
func TestHelpModalScrollThreshold(t *testing.T) {
	m := newModel("T", "hi")
	m.width = 80
	threshold := len(m.helpLines) + 11
	m.height = threshold
	if _, innerH := m.helpInnerDims(); innerH != len(m.helpLines) {
		t.Fatalf("H=%d must show all %d help lines (no scroll), got innerH=%d", threshold, len(m.helpLines), innerH)
	}
	m.height = threshold - 1
	if _, innerH := m.helpInnerDims(); innerH >= len(m.helpLines) {
		t.Fatalf("H=%d must scroll (innerH < %d), got %d", threshold-1, len(m.helpLines), innerH)
	}
}

// TestHelpModalScrollbarUsesMantle verifies that when the help modal renders a
// horizontal scrollbar (needH=true), the scrollbar row uses colMantle
// (#181825 → 48;2;24;24;37) as its background and NOT colCodeBg
// (#282C41 → 48;2;40;44;65).
func TestHelpModalScrollbarUsesMantle(t *testing.T) {
	// width=30 gives cw=26, innerW=12, contentW=42 → needH=true.
	m := newModel("T", "hi")
	m.width, m.height = 30, 24
	m.helpMode = true

	innerW, _ := m.helpInnerDims()
	contentW := MaxWideWidth(m.helpLines)
	if contentW <= innerW {
		t.Skipf("needH not triggered at this size (contentW=%d, innerW=%d); adjust test dimensions", contentW, innerW)
	}

	out := m.helpModal()

	const mantleBgParams = "48;2;24;24;37"  // colMantle #181825
	const codeBgParams   = "48;2;40;44;65"  // colCodeBg #282C41
	if !strings.Contains(out, mantleBgParams) {
		t.Fatalf("help modal scrollbar row must use colMantle bg (%s), but sequence not found in output", mantleBgParams)
	}
	if strings.Contains(out, codeBgParams) {
		t.Fatalf("help modal scrollbar row must NOT use colCodeBg bg (%s), but sequence was found in output", codeBgParams)
	}
}
