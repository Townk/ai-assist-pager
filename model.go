package main

import (
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// spinTickMsg drives the spinner animation/timer while thinking.
type spinTickMsg struct{}

func (m model) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return spinTickMsg{} })
}

// renderInterval bounds how often streamed text is re-rendered. A stream can
// deliver many small chunks per second; rather than reflow (parse + highlight
// the whole accumulated buffer) and repaint on every chunk — which saturates
// the event loop and stutters — chunks are appended cheaply and a single
// reflow is coalesced per interval (~30fps).
const renderInterval = 33 * time.Millisecond

// renderTickMsg flushes any pending streamed text into a reflow.
type renderTickMsg struct{}

func (m model) renderTickCmd() tea.Cmd {
	return tea.Tick(renderInterval, func(time.Time) tea.Msg { return renderTickMsg{} })
}

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

	// streaming + thinking
	thinking     bool
	thinkLabel   string
	defaultLabel string
	spinFrame    int
	spinTicks    int // 100ms ticks within the current thinking session (seconds = /10)
	streaming    bool
	follow       bool      // auto-scroll to bottom while streaming
	reader       io.Reader // input stream source (set by main); nil in tests/static
	parser       *streamParser

	dirty           bool // streamed text appended since the last reflow
	renderScheduled bool // a coalesced render tick is already pending
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
	return model{
		harness:      harness,
		md:           md,
		width:        80,
		height:       24,
		helpLines:    buildHelpLines(),
		defaultLabel: "Working…",
		follow:       true,
	}
}

func (m model) Init() tea.Cmd {
	if m.reader == nil {
		return nil
	}
	cmds := []tea.Cmd{readStream(m.reader, m.parser)}
	if m.thinking {
		cmds = append(cmds, m.tickCmd())
	}
	return tea.Batch(cmds...)
}

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

// flushRender re-renders the accumulated stream buffer if any text is pending,
// pinning the view to the bottom while following. No-op when nothing is dirty,
// so it's cheap to call from the render tick and on EOF.
func (m *model) flushRender() {
	if !m.dirty {
		return
	}
	m.reflow()
	if m.follow {
		m.yOff = len(m.lines) // clampScroll caps to the bottom
		m.clampScroll()
	}
	m.dirty = false
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
	case streamEventsMsg:
		startedThinking := false
		for _, ev := range msg.events {
			switch e := ev.(type) {
			case textEvent:
				m.md += e.text // cheap append; reflow is coalesced (renderTickMsg)
				m.dirty = true
				m.thinking = false
			case thinkEvent:
				label := e.label
				if label == "" {
					label = m.defaultLabel
				}
				if !m.thinking { // new thinking session: reset the timer
					m.thinking = true
					m.spinFrame = 0
					m.spinTicks = 0
					startedThinking = true
				}
				m.thinkLabel = label
			}
		}
		if msg.eof {
			m.flushRender() // render whatever's pending immediately
			m.streaming = false
			m.thinking = false
			return m, nil
		}
		cmds := []tea.Cmd{readStream(m.reader, m.parser)}
		if startedThinking {
			cmds = append(cmds, m.tickCmd())
		}
		// Coalesce the (expensive) whole-buffer reflow to renderInterval instead
		// of reflowing on every chunk. Schedule at most one tick at a time.
		if m.dirty && !m.renderScheduled {
			m.renderScheduled = true
			cmds = append(cmds, m.renderTickCmd())
		}
		return m, tea.Batch(cmds...)
	case renderTickMsg:
		m.renderScheduled = false
		m.flushRender()
		return m, nil
	case spinTickMsg:
		if !m.thinking {
			return m, nil
		}
		m.spinFrame++
		m.spinTicks++
		return m, m.tickCmd()
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

func bi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// helpTextDims returns the modal's visible help-text area (cols x rows) and
// whether each scrollbar is shown. The title now scrolls with the content
// (m.helpLines includes it), so the modal area (m.height-4) holds, top to
// bottom: border(1) + padTop(1) + text rows + padBottom(1) + border(1) = text+4.
// Horizontally the box is capped to cw-8 (4-col margins) and laid out as
// border(1) + leftPad(2) + text + gap(2) + vbar(needV?1:0) + border(1): the bar
// sits flush against the right border with a 2-col gap from the text, so the
// text budget is cw-14, minus one more column when the vbar is shown. The
// horizontal bar (when needH) takes one text row. All dims floored at 1.
func (m model) helpTextDims() (textW, textH int, needV, needH bool) {
	cw := m.contentWidth()
	contentMaxW := MaxWideWidth(m.helpLines)
	maxRows := m.height - 8
	if maxRows < 1 {
		maxRows = 1
	}
	// Two passes resolve the interaction between the bars: reserving the hbar row
	// can tip vertical overflow, and showing the vbar narrows the text budget.
	for pass := 0; pass < 2; pass++ {
		available := maxRows - bi(needH) // rows left for text after the hbar
		if available < 1 {
			available = 1
		}
		needV = len(m.helpLines) > available
		maxTextW := cw - 14 - bi(needV)
		if maxTextW < 1 {
			maxTextW = 1
		}
		needH = contentMaxW > maxTextW
	}
	// Visible dims: content-sized, capped to the available area.
	textH = maxRows - bi(needH)
	if textH > len(m.helpLines) {
		textH = len(m.helpLines)
	}
	if textH < 1 {
		textH = 1
	}
	textW = cw - 14 - bi(needV)
	if textW > contentMaxW {
		textW = contentMaxW
	}
	if textW < 1 {
		textW = 1
	}
	return textW, textH, needV, needH
}

func (m *model) clampHelpScroll() {
	textW, textH, _, _ := m.helpTextDims()
	maxY := len(m.helpLines) - textH
	if maxY < 0 {
		maxY = 0
	}
	if m.helpYOff > maxY {
		m.helpYOff = maxY
	}
	if m.helpYOff < 0 {
		m.helpYOff = 0
	}
	maxX := MaxWideWidth(m.helpLines) - textW
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

func helpInnerH(m model) int { _, h, _, _ := m.helpTextDims(); return h }
func helpInnerW(m model) int { w, _, _, _ := m.helpTextDims(); return w }
func helpHalf(m model) int   { if h := helpInnerH(m) / 2; h > 1 { return h }; return 1 }
func helpPage(m model) int   { if h := helpInnerH(m); h > 1 { return h }; return 1 }
func helpHalfW(m model) int  { if w := helpInnerW(m) / 2; w > 1 { return w }; return 1 }

// mantleBg is the ANSI truecolor background sequence for colMantle, used to
// band each interior row so the modal background is uniform throughout.
const mantleBg = "\x1b[48;2;24;24;37m" // #181825 = R24 G24 B37

// helpModal renders the centered keybinding modal over the body region.
func (m model) helpModal() string {
	cw := m.contentWidth()
	// Modal area = rows 3..H-2 (m.height-4 lines): a 2-line top margin (blank +
	// title) above, and bottomPad + status bar below. The box is centered here.
	bodyW, bodyH := cw, m.height-4
	if bodyH < 1 {
		bodyH = 1
	}
	textW, textH, needV, needH := m.helpTextDims()
	contentW := MaxWideWidth(m.helpLines)

	// All padding is applied manually (the box uses Padding(0,0)) so both
	// scrollbars run flush to their borders. Each row is leftPad(2) + text +
	// gap(2) + vbar(1 when needV). Rows top to bottom: top pad, text rows, bottom
	// pad, then the hbar (when needH) flush against the bottom border with the
	// bottom pad as its gap above. The vbar occupies the rightmost column on every
	// row, so it runs from the top border to the bottom border.
	windowed := Window(m.helpLines, m.helpXOff, m.helpYOff, textW, textH)
	trackH := textH + 2 + bi(needH) // top pad + text rows + bottom pad + optional hbar
	vpos, vsize := vthumbTrack(len(m.helpLines), textH, trackH, m.helpYOff)
	vbar := func(trackRow int) string {
		if !needV {
			return ""
		}
		glyph, col := "│", colSurface0
		if trackRow >= vpos && trackRow < vpos+vsize {
			glyph, col = "┃", colOverlay1
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(col)).Render(glyph)
	}
	// band re-injects the modal bg after every inner color reset so plain gaps and
	// reset segments keep the modal background instead of the terminal's.
	blank := strings.Repeat(" ", textW)
	row := func(text string, trackRow int) string {
		return band("  "+text+"  "+vbar(trackRow), mantleBg, 0)
	}
	var body []string
	tr := 0
	body = append(body, row(blank, tr)) // top pad row
	tr++
	for _, w := range windowed {
		body = append(body, row(padTo(w, textW), tr))
		tr++
	}
	body = append(body, row(blank, tr)) // bottom pad / gap above the hbar
	tr++
	if needH {
		body = append(body, row(hscrollbarRow(contentW, m.helpXOff, textW, colMantle), tr)) // hbar flush to bottom border
	}

	content := strings.Join(body, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colSurface1)).
		BorderBackground(lipgloss.Color(colMantle)).
		Background(lipgloss.Color(colMantle)).
		Padding(0, 0).
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
		sb.WriteString("\n")                     // row 1: blank
		sb.WriteString("  " + m.header() + "\n") // row 2: title (2-line top margin)
		sb.WriteString(m.helpModal())            // rows 3..H-2: modal area = m.height-4 rows
		sb.WriteString("\n")                     // end the modal's last line
		sb.WriteString("\n")                     // bottom pad (row H-1)
		sb.WriteString("  " + m.statusBar())     // row H: status bar
	} else {
		rows := Window(m.lines, m.xOff, m.yOff, cw, m.body())
		pos, size := vthumb(len(m.lines), m.body(), m.yOff)
		sb.WriteString("\n")
		sb.WriteString("  " + m.header() + "\n")
		sb.WriteString("\n")
		spinRow := -1
		if m.thinking {
			// Spinner sits just below the last real content line visible from the
			// top of the body (or the first body row when empty), within the body
			// region. len(m.lines)-m.yOff gives the number of real content rows
			// still ahead of the viewport top; that is the row index right after
			// the last visible content line.
			spinRow = len(m.lines) - m.yOff
			if spinRow < 0 {
				spinRow = 0
			}
			if spinRow > m.body()-1 {
				spinRow = m.body() - 1
			}
		}
		for i := 0; i < m.body(); i++ {
			if i == spinRow {
				sb.WriteString("  " + padTo(spinnerLine(m.spinFrame, m.thinkLabel, m.spinTicks/10), cw) + vscrollCell(spinRow, pos, size) + "\n")
				continue
			}
			if i < len(rows) {
				row := rows[i]
				idx := m.yOff + i
				if idx >= 0 && idx < len(m.lines) && m.lines[idx].HBar > 0 {
					row = hscrollbarRow(m.lines[idx].HBar, m.xOff, cw, colCodeBg)
				}
				sb.WriteString("  " + padTo(row, cw) + vscrollCell(i, pos, size) + "\n")
			} else {
				sb.WriteString("\n")
			}
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
