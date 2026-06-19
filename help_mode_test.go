package main

import (
	"strings"
	"testing"

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

func TestHelpInnerDims(t *testing.T) {
	w, h := helpInnerDims(120, 40)
	if w < 1 || h < 1 {
		t.Fatalf("generous pane gave non-positive dims: %d x %d", w, h)
	}
	w, h = helpInnerDims(8, 5) // tiny pane
	if w < 1 || h < 1 {
		t.Fatalf("tiny pane must still give ≥1x1: %d x %d", w, h)
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
	_, innerH := helpInnerDims(m.width, m.height)
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
		if h := lipgloss.Height(out); h != m.body() {
			t.Fatalf("%dx%d: modal height %d != body %d", d[0], d[1], h, m.body())
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
