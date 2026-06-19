package main

import (
	"encoding/base64"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type model struct {
	harness    string
	md         string
	lines      []Line
	buttons    []Button
	width      int
	height     int
	xOff       int
	yOff       int
	fifoPath   string
	hintMode   bool
	hintLabels map[string]Button
}

// emitAction appends "<kind>\t<base64 payload>\n" to the actions FIFO. No-op when
// no FIFO is set (standalone/sample). O_APPEND|O_CREATE so a regular file works in
// tests and a real FIFO opened by a reader also works.
func (m model) emitAction(b Button) {
	if m.fifoPath == "" {
		return
	}
	f, err := os.OpenFile(m.fifoPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(b.Kind + "\t" + base64.StdEncoding.EncodeToString([]byte(b.Payload)) + "\n")
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
	h := m.height - headerRows - hintRows - 3 // subtract leading blank + top and bottom padding rows
	if h < 1 {
		h = 1
	}
	return h
}

func (m *model) reflow() {
	m.lines, m.buttons = Render(m.md, m.contentWidth())
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

// bodyTop is the screen row (0-based) of the first body line.
// Layout: leading blank(1) + header(1) + top-pad(1) = row 3.
const bodyTop = 1 + headerRows + 1

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.reflow()
		return m, nil
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if b, ok := buttonAt(m.buttons, msg.X, msg.Y, m.yOff, bodyTop); ok {
				m.emitAction(b)
			}
		}
		return m, nil
	case tea.KeyPressMsg:
		// Hint mode: resolve the pending label before any normal nav.
		if m.hintMode {
			switch msg.String() {
			case "esc":
				m.hintMode = false
				m.hintLabels = nil
			default:
				if b, ok := m.hintLabels[msg.String()]; ok {
					m.emitAction(b)
				}
				m.hintMode = false
				m.hintLabels = nil
			}
			return m, nil
		}
		// Leader: Space enters hint mode over the visible buttons.
		if msg.String() == " " {
			var visible []Button
			for _, b := range m.buttons {
				if b.Line >= m.yOff && b.Line < m.yOff+m.body() {
					visible = append(visible, b)
				}
			}
			if len(visible) > 0 {
				m.hintLabels = assignHintLabels(visible)
				m.hintMode = true
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		// Vertical: line
		case "down", "j":
			m.yOff++
		case "up", "k":
			m.yOff--
		// Vertical: half-page
		case "ctrl+d":
			half := m.body() / 2
			if half < 1 {
				half = 1
			}
			m.yOff += half
		case "ctrl+u":
			half := m.body() / 2
			if half < 1 {
				half = 1
			}
			m.yOff -= half
		// Vertical: full-page
		case "ctrl+f", "pgdown":
			m.yOff += m.body()
		case "ctrl+b", "pgup":
			m.yOff -= m.body()
		// Vertical: top/bottom
		case "g", "home":
			m.yOff = 0
		case "G", "end":
			m.yOff = len(m.lines)
		// Horizontal: 1-col
		case "right", "l":
			m.xOff++
		case "left", "h":
			m.xOff--
		// Horizontal: half-width jump
		case "L":
			hstep := m.contentWidth() / 2
			if hstep < 1 {
				hstep = 1
			}
			m.xOff += hstep
		case "H":
			hstep := m.contentWidth() / 2
			if hstep < 1 {
				hstep = 1
			}
			m.xOff -= hstep
		// Horizontal: home/end
		case "0", "^":
			m.xOff = 0
		case "$":
			m.xOff = MaxWideWidth(m.lines) // clampScroll will cap it
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
		Render("j/k ↑↓  ^d/^u half  ^f/^b page  h/l ←→  H/L 0/$ horiz  g/G  q quit  Space hints")
}

// hintLegend builds the compact legend shown at the bottom when hint mode is
// active, e.g. "[a] play  [s] copy  [d] play". The alphabet order is preserved
// by iterating hintAlphabet rather than the map.
func (m model) hintLegend() string {
	labelStyle := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(colBase)).
		Background(lipgloss.Color(colYellow))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
	var parts []string
	for _, ch := range hintAlphabet {
		label := string(ch)
		if b, ok := m.hintLabels[label]; ok {
			parts = append(parts, labelStyle.Render(label)+" "+keyStyle.Render(b.Kind))
		}
	}
	prefix := lipgloss.NewStyle().Foreground(lipgloss.Color(colYellow)).Bold(true).Render("HINT ")
	return prefix + strings.Join(parts, keyStyle.Render("  "))
}

func (m model) View() tea.View {
	cw := m.contentWidth()
	rows := Window(m.lines, m.xOff, m.yOff, cw, m.body())
	var sb strings.Builder
	// Empty line before the title.
	sb.WriteString("\n")
	// Header (left-padded)
	sb.WriteString("  " + m.header() + "\n")
	// Top padding blank
	sb.WriteString("\n")
	// Rows 3..H-2: body (each left-padded)
	for _, row := range rows {
		sb.WriteString("  " + row + "\n")
	}
	// Row H-1: bottom padding blank
	sb.WriteString("\n")
	// Row H: hint (left-padded); replaced by legend when hint mode is active.
	if m.hintMode {
		sb.WriteString("  " + m.hintLegend())
	} else {
		sb.WriteString("  " + m.hint())
	}
	v := tea.NewView(sb.String())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// staticRender returns the full rendered content (no scroll chrome) for
// printing to the pane on exit, so the docked pane parks showing the reply.
// Content is wrapped at contentWidth and left-padded with 2 spaces to match
// the interactive View().
func (m model) staticRender() string {
	cw := m.contentWidth()
	lines, _ := Render(m.md, cw)
	var sb strings.Builder
	sb.WriteString("  " + m.header() + "\n")
	sb.WriteString("\n")
	for _, l := range lines {
		sb.WriteString("  " + l.Text + "\n")
	}
	return sb.String()
}
