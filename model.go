package main

import (
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
	helpMode   bool
	helpLines  []Line
	helpYOff   int
	helpXOff   int
}

// emitAction appends a record framed as "<kind>US<payload>RS" to the actions
// FIFO, where US (0x1f, Unit Separator) separates kind from payload and RS
// (0x1e, Record Separator) terminates the record. Payload is written byte-exact
// (no encoding). No-op when no FIFO is set (standalone/sample). O_APPEND|O_CREATE
// so a regular file works in tests and a real FIFO opened by a reader also works.
// O_NONBLOCK prevents blocking the bubbletea event loop when no reader is attached
// to the FIFO (returns ENXIO).
func (m model) emitAction(b Button) {
	if m.fifoPath == "" {
		return
	}
	f, err := os.OpenFile(m.fifoPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(b.Kind + "\x1f" + b.Payload + "\x1e")
}

func newModel(harness, md string) model {
	return model{harness: harness, md: md, width: 80, height: 24, helpLines: buildHelpLines()}
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
		m.clampHelpScroll()
		return m, nil
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if b, ok := buttonAt(m.buttons, msg.X, msg.Y, m.yOff, bodyTop); ok {
				m.emitAction(b)
			}
		}
		return m, nil
	case tea.KeyPressMsg:
		// Help overlay: resolve before hint/normal handling.
		if m.helpMode {
			switch msg.String() {
			case "esc", "q", "?":
				m.helpMode = false
			case "down", "j":
				m.helpYOff++
			case "up", "k":
				m.helpYOff--
			case "ctrl+d":
				m.helpYOff += helpHalf(m)
			case "ctrl+u":
				m.helpYOff -= helpHalf(m)
			case "ctrl+f", "pgdown":
				m.helpYOff += helpPage(m)
			case "ctrl+b", "pgup":
				m.helpYOff -= helpPage(m)
			case "g", "home":
				m.helpYOff = 0
			case "G", "end":
				m.helpYOff = len(m.helpLines)
			case "right", "l":
				m.helpXOff++
			case "left", "h":
				m.helpXOff--
			case "L":
				m.helpXOff += helpHalfW(m)
			case "H":
				m.helpXOff -= helpHalfW(m)
			case "0", "^":
				m.helpXOff = 0
			case "$":
				m.helpXOff = MaxWideWidth(m.helpLines)
			}
			m.clampHelpScroll()
			return m, nil
		}
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
		case "?":
			m.helpMode = true
			m.helpYOff = 0
			m.helpXOff = 0
			return m, nil
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

// helpInnerDims is the modal's inner content area (cols x rows) for a pane of
// width x height: pane minus a ~2-cell outer margin, the border (2), the inner
// padding (8 horizontal / 4 vertical) and the title row (1). Floored at 1.
func helpInnerDims(width, height int) (innerW, innerH int) {
	innerW = width - 4 - 2 - 8  // margin + border + h-padding
	innerH = height - 4 - 2 - 4 - 1 - 1 // margin + border + v-padding + title + body() offset
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	return innerW, innerH
}

func (m *model) clampHelpScroll() {
	innerW, innerH := helpInnerDims(m.width, m.height)
	maxY := len(m.helpLines) - innerH
	if maxY < 0 {
		maxY = 0
	}
	if m.helpYOff > maxY {
		m.helpYOff = maxY
	}
	if m.helpYOff < 0 {
		m.helpYOff = 0
	}
	maxX := MaxWideWidth(m.helpLines) - innerW
	if maxX < 0 {
		maxX = 0
	}
	if m.helpXOff > maxX {
		m.helpXOff = maxX
	}
	if m.helpXOff < 0 {
		m.helpXOff = 0
	}
}

// statusBar is the slim, mode-aware bottom hint.
func (m model) statusBar() string {
	st := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
	if m.hintMode || m.helpMode {
		return st.Render("\U000F12B7: cancel")
	}
	return st.Render("\U000F1050: action • \U000F12B7: close • ?: keys")
}

func helpInnerH(m model) int { _, h := helpInnerDims(m.width, m.height); return h }
func helpInnerW(m model) int { w, _ := helpInnerDims(m.width, m.height); return w }
func helpHalf(m model) int   { if h := helpInnerH(m) / 2; h > 1 { return h }; return 1 }
func helpPage(m model) int   { if h := helpInnerH(m); h > 1 { return h }; return 1 }
func helpHalfW(m model) int  { if w := helpInnerW(m) / 2; w > 1 { return w }; return 1 }

// helpModal renders the centered keybinding modal over the body region.
func (m model) helpModal() string {
	cw := m.contentWidth()
	bodyW, bodyH := cw, m.body()
	innerW, innerH := helpInnerDims(m.width, m.height)

	// The box chrome is: border(2) + v-padding(4) + title(1) = 7 rows.
	// Clamp innerH so the box never overflows bodyH.
	maxInnerH := bodyH - 7
	if maxInnerH < 0 {
		maxInnerH = 0
	}
	if innerH > maxInnerH {
		innerH = maxInnerH
	}
	if innerH < 0 {
		innerH = 0
	}

	contentW := MaxWideWidth(m.helpLines)
	needV := innerH > 0 && len(m.helpLines) > innerH
	needH := innerH > 0 && contentW > innerW
	rowsW := innerW
	if needV {
		rowsW -= 2 // vscrollCell returns " " + glyph = 2 display columns
	}
	rowsH := innerH
	if needH {
		rowsH-- // leave a row for the horizontal scrollbar
	}
	if rowsW < 1 {
		rowsW = 1
	}
	if rowsH < 1 && innerH > 0 {
		rowsH = 1
	}

	windowed := Window(m.helpLines, m.helpXOff, m.helpYOff, rowsW, rowsH)
	vpos, vsize := vthumb(len(m.helpLines), rowsH, m.helpYOff)
	var body []string
	for i, row := range windowed {
		line := padTo(row, rowsW)
		if needV {
			line += vscrollCell(i, vpos, vsize) // " " + glyph
		}
		body = append(body, line)
	}
	if needH {
		// hscrollbarRow renders a code-bg bar; here we just need ─/━ at innerW.
		body = append(body, hscrollbarRow(contentW, m.helpXOff, innerW))
	}

	title := lipgloss.NewStyle().Foreground(lipgloss.Color(colMauve)).Bold(true).
		Render(" Keybindings ")
	content := title
	if len(body) > 0 {
		content += "\n" + strings.Join(body, "\n")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colMauve)).
		Background(lipgloss.Color(colSurface0)).
		Padding(2, 4).
		Render(content)

	out := lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, box)
	lines := strings.Split(out, "\n")
	if len(lines) > bodyH {
		lines = lines[:bodyH]
	}
	clipStyle := lipgloss.NewStyle().MaxWidth(bodyW)
	for i, line := range lines {
		if lipgloss.Width(line) > bodyW {
			lines[i] = padTo(clipStyle.Render(line), bodyW)
		}
	}
	return strings.Join(lines, "\n")
}

// viewString assembles the full rendered frame as a plain string. View wraps
// this in tea.NewView so that tests can call viewString() directly without
// needing to extract Content from a tea.View.
func (m model) viewString() string {
	cw := m.contentWidth()
	var sb strings.Builder

	if m.hintMode {
		// Labels float on the line above each button (or below when the line
		// above is scrolled off the top).
		labelsByRow := map[int]map[int]string{}
		for label, b := range m.hintLabels {
			row := b.Line - 1
			if row < m.yOff {
				row = b.Line + 1
			}
			if labelsByRow[row] == nil {
				labelsByRow[row] = map[int]string{}
			}
			labelsByRow[row][b.Col] = label
		}
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
		lab := lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(colHintLabelFg)).
			Background(lipgloss.Color(colHintLabelBg))

		// Button glyph columns per tab line — given the hint-label dark-red bg.
		buttonColsByRow := map[int]map[int]bool{}
		for _, b := range m.buttons {
			if buttonColsByRow[b.Line] == nil {
				buttonColsByRow[b.Line] = map[int]bool{}
			}
			buttonColsByRow[b.Line][b.Col] = true
		}

		rows := Window(m.lines, m.xOff, m.yOff, cw, m.body())
		pos, size := vthumb(len(m.lines), m.body(), m.yOff)
		sb.WriteString("\n")
		sb.WriteString("  " + m.header() + "\n")
		sb.WriteString("\n")
		for i, row := range rows {
			idx := m.yOff + i
			var base string
			if idx >= 0 && idx < len(m.lines) && m.lines[idx].Code {
				base = hintCodeRow(row, cw, buttonColsByRow[idx]) // fill + dark-red button cells
			} else {
				base = dim.Render(padTo(strip(row), cw))
			}
			base = overlayLabels(base, labelsByRow[idx], lab)
			sb.WriteString("  " + base + vscrollCell(i, pos, size) + "\n")
		}
		sb.WriteString("\n")
		sb.WriteString("  " + m.statusBar())
	} else if m.helpMode {
		sb.WriteString("\n")
		sb.WriteString("  " + m.header() + "\n")
		sb.WriteString("\n")
		// The modal occupies the body region (m.body() rows).
		sb.WriteString(m.helpModal())
		sb.WriteString("\n") // end the modal's last line
		sb.WriteString("\n") // bottom pad (mirror the normal branch)
		sb.WriteString("  " + m.statusBar())
	} else {
		rows := Window(m.lines, m.xOff, m.yOff, cw, m.body())
		pos, size := vthumb(len(m.lines), m.body(), m.yOff)
		sb.WriteString("\n")
		sb.WriteString("  " + m.header() + "\n")
		sb.WriteString("\n")
		for i, row := range rows {
			idx := m.yOff + i
			if idx >= 0 && idx < len(m.lines) && m.lines[idx].HBar > 0 {
				row = hscrollbarRow(m.lines[idx].HBar, m.xOff, cw)
			}
			sb.WriteString("  " + padTo(row, cw) + vscrollCell(i, pos, size) + "\n")
		}
		sb.WriteString("\n")
		sb.WriteString("  " + m.statusBar())
	}

	return sb.String()
}

func (m model) View() tea.View {
	v := tea.NewView(m.viewString())
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
