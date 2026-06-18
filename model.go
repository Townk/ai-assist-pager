package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type model struct {
	harness string
	md      string
	lines   []Line
	width   int
	height  int
	xOff    int
	yOff    int
}

func newModel(harness, md string) model {
	return model{harness: harness, md: md, width: 80, height: 24}
}

func (m model) Init() tea.Cmd { return nil }

// headerRows is the height the header takes (title only; top padding provides
// the gap between header and body).
const headerRows = 1

// hintRows is the height the bottom key-hint takes.
const hintRows = 1

// contentWidth returns the render/scroll width: full width minus 2-col left
// and 2-col right margins (floored at 1).
func (m *model) contentWidth() int {
	w := m.width - 4
	if w < 1 {
		w = 1
	}
	return w
}

// body returns the number of visible body rows.
// Layout: header(1) + topPad(1) + body(H-4) + botPad(1) + hint(1) = H.
func (m *model) body() int {
	h := m.height - headerRows - hintRows - 2 // subtract top and bottom padding rows
	if h < 1 {
		h = 1
	}
	return h
}

func (m *model) reflow() {
	m.lines = Render(m.md, m.contentWidth())
	m.clampScroll()
}

func (m *model) clampScroll() {
	maxY := len(m.lines) - m.body()
	if maxY < 0 {
		maxY = 0
	}
	if m.yOff > maxY {
		m.yOff = maxY
	}
	if m.yOff < 0 {
		m.yOff = 0
	}
	maxX := MaxWideWidth(m.lines) - m.contentWidth()
	if maxX < 0 {
		maxX = 0
	}
	if m.xOff > maxX {
		m.xOff = maxX
	}
	if m.xOff < 0 {
		m.xOff = 0
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.reflow()
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "down", "j":
			m.yOff++
		case "up", "k":
			m.yOff--
		case "right", "l":
			m.xOff++
		case "left", "h":
			m.xOff--
		case "pgdown", "ctrl+d":
			m.yOff += m.body()
		case "pgup", "ctrl+u":
			m.yOff -= m.body()
		case "g", "home":
			m.yOff = 0
		case "G", "end":
			m.yOff = len(m.lines)
		}
		m.clampScroll()
		return m, nil
	}
	return m, nil
}

func (m model) header() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colMauve)).Bold(true).
		Render(strings.Repeat("▓", 3) + " ai-assist — " + m.harness)
}

func (m model) hint() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0)).
		Render("  ↑↓ scroll • ←→ code/tables • g/G top/bottom • q quit")
}

func (m model) View() tea.View {
	cw := m.contentWidth()
	rows := Window(m.lines, m.xOff, m.yOff, cw, m.body())
	var sb strings.Builder
	// Row 1: header (left-padded)
	sb.WriteString("  " + m.header() + "\n")
	// Row 2: top padding blank
	sb.WriteString("\n")
	// Rows 3..H-2: body (each left-padded)
	for _, row := range rows {
		sb.WriteString("  " + row + "\n")
	}
	// Row H-1: bottom padding blank
	sb.WriteString("\n")
	// Row H: hint (left-padded)
	sb.WriteString("  " + m.hint())
	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

// staticRender returns the full rendered content (no scroll chrome) for
// printing to the pane on exit, so the docked pane parks showing the reply.
// Content is wrapped at contentWidth and left-padded with 2 spaces to match
// the interactive View().
func (m model) staticRender() string {
	cw := m.contentWidth()
	lines := Render(m.md, cw)
	var sb strings.Builder
	sb.WriteString("  " + m.header() + "\n")
	sb.WriteString("\n")
	for _, l := range lines {
		sb.WriteString("  " + l.Text + "\n")
	}
	return sb.String()
}
