package main

import (
	"encoding/base64"
	"os"
	"strings"
	"syscall"

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
// tests and a real FIFO opened by a reader also works. O_NONBLOCK prevents blocking
// the bubbletea event loop when no reader is attached to the FIFO (returns ENXIO).
func (m model) emitAction(b Button) {
	if m.fifoPath == "" {
		return
	}
	f, err := os.OpenFile(m.fifoPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE|syscall.O_NONBLOCK, 0o600)
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
		// Leader: Space enters hint mode over the visible buttons. bubbletea v2
		// (ultraviolet) reports the space key as "space", not " ".
		if s := msg.String(); s == "space" || s == " " {
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

// hintRow renders one body row for hint mode: the plain (already ANSI-stripped)
// text dimmed to dim, with label chars overlaid at their display columns
// (labels[col] = label) in the lab standout style. The row is padded to width.
// Column == rune index here (label rows are blank/space above a tab, so this is
// exact for the common case).
func hintRow(plain string, labels map[int]string, width int, dim, lab lipgloss.Style) string {
	runes := []rune(plain)
	for len(runes) < width {
		runes = append(runes, ' ')
	}
	if len(runes) > width {
		runes = runes[:width]
	}
	var sb strings.Builder
	var run []rune
	flush := func() {
		if len(run) > 0 {
			sb.WriteString(dim.Render(string(run)))
			run = run[:0]
		}
	}
	for i, r := range runes {
		if lbl, ok := labels[i]; ok {
			flush()
			sb.WriteString(lab.Render(lbl))
		} else {
			run = append(run, r)
		}
	}
	flush()
	return sb.String()
}

func (m model) View() tea.View {
	cw := m.contentWidth()
	var sb strings.Builder

	if m.hintMode {
		// Flash.nvim-style in-place overlay: dim all body rows, float each label
		// char on the line directly above its button (fallback: same line when
		// the line above is scrolled off the top of the viewport).

		// Build labelsByRow: lineIdx → col → label.
		labelsByRow := map[int]map[int]string{}
		for label, b := range m.hintLabels {
			row := b.Line - 1
			if row < m.yOff {
				row = b.Line
			}
			if labelsByRow[row] == nil {
				labelsByRow[row] = map[int]string{}
			}
			labelsByRow[row][b.Col] = label
		}

		dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
		lab := lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(colBase)).
			Background(lipgloss.Color(colMauve))

		rows := Window(m.lines, m.xOff, m.yOff, cw, m.body())

		// Empty line before the title.
		sb.WriteString("\n")
		// Header (left-padded, not dimmed).
		sb.WriteString("  " + m.header() + "\n")
		// Top padding blank.
		sb.WriteString("\n")
		// Body rows: dim + overlay labels.
		for i, row := range rows {
			lineIdx := m.yOff + i
			plain := strip(row)
			sb.WriteString("  " + hintRow(plain, labelsByRow[lineIdx], cw, dim, lab) + "\n")
		}
		// Bottom padding blank.
		sb.WriteString("\n")
		// Bottom prompt (dim).
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0)).Render("press a label • Esc cancel"))
	} else {
		rows := Window(m.lines, m.xOff, m.yOff, cw, m.body())
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
		// Row H: hint (left-padded).
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
