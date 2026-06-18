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

// headerRows is the height the header takes (title + blank).
const headerRows = 2

// hintRows is the height the bottom key-hint takes.
const hintRows = 1

func (m *model) body() int {
	h := m.height - headerRows - hintRows
	if h < 1 {
		h = 1
	}
	return h
}

func (m *model) reflow() {
	m.lines = Render(m.md, m.width)
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
	maxX := MaxWideWidth(m.lines) - m.width
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
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(colMauve)).Bold(true).
		Render(strings.Repeat("▓", 3) + " ai-assist — " + m.harness)
	return title + "\n"
}

func (m model) hint() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0)).
		Render("  ↑↓ scroll • ←→ code/tables • g/G top/bottom • q quit")
}

func (m model) View() tea.View {
	rows := Window(m.lines, m.xOff, m.yOff, m.width, m.body())
	body := strings.Join(rows, "\n")
	v := tea.NewView(m.header() + body + "\n" + m.hint())
	v.AltScreen = true
	return v
}

// staticRender returns the full rendered content (no scroll chrome) for printing
// to the pane on exit, so the docked pane parks showing the reply.
func (m model) staticRender() string {
	lines := Render(m.md, m.width)
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = l.Text
	}
	return m.header() + strings.Join(parts, "\n") + "\n"
}
