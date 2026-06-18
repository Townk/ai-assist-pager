package main

import (
	"bytes"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// strip removes ANSI/CSI escape sequences so callers can measure or assert on
// the visible text. ESC introduces a sequence; for CSI ("ESC [") it consumes
// the parameter/intermediate bytes and the final byte (0x40–0x7e).
func strip(s string) string {
	var b strings.Builder
	const (
		normal = iota
		sawESC
		inCSI
	)
	state := normal
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch state {
		case normal:
			if c == 0x1b {
				state = sawESC
			} else {
				b.WriteByte(c)
			}
		case sawESC:
			if c == '[' {
				state = inCSI
			} else {
				state = normal // non-CSI escape; sequence over
			}
		case inCSI:
			if c >= 0x40 && c <= 0x7e {
				state = normal // final byte, consumed
			}
		}
	}
	return b.String()
}

type renderer struct {
	src   []byte
	width int
	lines []Line
}

// Render parses markdown and returns tagged, laid-out lines for a given pane width.
func Render(md string, width int) []Line {
	if width < 1 {
		width = 1
	}
	src := []byte(md)
	gm := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := gm.Parser().Parse(text.NewReader(src))
	r := &renderer{src: src, width: width}
	r.block(doc, 0)
	return r.lines
}

// block walks the children of n, rendering each block-level node. indent is the
// current left indentation (used by nested lists / quotes).
func (r *renderer) block(n ast.Node, indent int) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch node := c.(type) {
		case *ast.Heading:
			prefix := strings.Repeat("▓", 3) + " "
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(headingColor(node.Level))).Bold(true)
			r.emitProse(style.Render(prefix+r.inline(node)), indent)
			r.blank()
		case *ast.Paragraph, *ast.TextBlock:
			r.emitProse(r.inline(c), indent)
			if _, ok := c.(*ast.Paragraph); ok {
				r.blank()
			}
		case *ast.List:
			r.list(node, indent)
			r.blank()
		case *ast.ThematicBreak:
			rule := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0)).
				Render(strings.Repeat("─", r.width-indent))
			r.emitProse(rule, indent)
			r.blank()
		case *ast.FencedCodeBlock:
			r.code(node)
			r.blank()
		case *ast.CodeBlock:
			r.code(node)
			r.blank()
		case *ast.Blockquote:
			r.quote(node, indent)
			r.blank()
		case *extast.Table:
			r.table(node)
			r.blank()
		default:
			// Fallback for block types without an explicit case (e.g. HTML blocks):
			// render their inline text so nothing is silently dropped.
			if t := strings.TrimRight(r.inline(c), "\n"); t != "" {
				r.emitProse(t, indent)
			}
		}
	}
	r.trimTrailingBlank()
}

func headingColor(level int) string {
	switch level {
	case 1:
		return colMauve
	case 2:
		return colPeach
	case 3:
		return colYellow
	default:
		return colGreen
	}
}

// list renders an ast.List, one marker per item, recursing for nested blocks.
func (r *renderer) list(l *ast.List, indent int) {
	i := 0
	for item := l.FirstChild(); item != nil; item = item.NextSibling() {
		marker := "• "
		if l.IsOrdered() {
			// l.Start is the first item's number; i is the zero-based offset, so
			// l.Start+i gives the correct display number for each item.
			marker = itoa(l.Start+i) + ". "
		}
		// First child of a list item is usually a paragraph/textblock.
		itemText := ""
		if fc := item.FirstChild(); fc != nil {
			itemText = r.inline(fc)
		}
		r.emitProse(marker+itemText, indent+2)
		// Nested lists inside this item.
		for sub := item.FirstChild(); sub != nil; sub = sub.NextSibling() {
			if nl, ok := sub.(*ast.List); ok {
				r.list(nl, indent+2)
			}
		}
		i++
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// inline renders the inline children of n into a single styled string.
func (r *renderer) inline(n ast.Node) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch node := c.(type) {
		case *ast.Text:
			b.WriteString(string(node.Segment.Value(r.src)))
			if node.HardLineBreak() || node.SoftLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.WriteString(string(node.Value))
		case *ast.Emphasis:
			st := lipgloss.NewStyle().Italic(true)
			if node.Level == 2 {
				st = lipgloss.NewStyle().Bold(true)
			}
			b.WriteString(st.Render(r.inline(node)))
		case *ast.CodeSpan:
			st := lipgloss.NewStyle().Foreground(lipgloss.Color(colPeach)).Background(lipgloss.Color(colCodeBg))
			b.WriteString(st.Render(" " + r.inlineText(node) + " "))
		case *ast.Link:
			st := lipgloss.NewStyle().Foreground(lipgloss.Color(colBlue)).Underline(true)
			b.WriteString(st.Render(r.inline(node)))
		case *extast.Strikethrough:
			b.WriteString(lipgloss.NewStyle().Strikethrough(true).Render(r.inline(node)))
		default:
			b.WriteString(r.inline(c))
		}
	}
	return b.String()
}

// inlineText extracts the raw text of an inline node (for code spans).
func (r *renderer) inlineText(n ast.Node) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.WriteString(string(t.Segment.Value(r.src)))
		} else {
			b.WriteString(r.inlineText(c))
		}
	}
	return b.String()
}

// emitProse wraps s to the pane width (minus indent) and appends Wide=false
// lines, each left-padded by indent spaces. lipgloss wraps ANSI-aware.
func (r *renderer) emitProse(s string, indent int) {
	w := r.width - indent
	if w < 1 {
		w = 1
	}
	wrapped := lipgloss.NewStyle().Width(w).Render(s)
	pad := strings.Repeat(" ", indent)
	for _, ln := range strings.Split(wrapped, "\n") {
		r.lines = append(r.lines, Line{Text: pad + ln, Wide: false})
	}
}

func (r *renderer) blank() { r.lines = append(r.lines, Line{Text: "", Wide: false}) }

func (r *renderer) trimTrailingBlank() {
	for len(r.lines) > 0 && strings.TrimSpace(strip(r.lines[len(r.lines)-1].Text)) == "" {
		r.lines = r.lines[:len(r.lines)-1]
	}
}

// table renders a GFM table via lipgloss/table at its natural width (no wrap),
// emitting Wide=true lines so it scrolls horizontally like a code block.
func (r *renderer) table(n *extast.Table) {
	var header []string
	var rows [][]string
	for row := n.FirstChild(); row != nil; row = row.NextSibling() {
		var cells []string
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cells = append(cells, strings.TrimSpace(strip(r.inline(cell))))
		}
		switch row.(type) {
		case *extast.TableHeader:
			header = cells
		default:
			rows = append(rows, cells)
		}
	}

	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))).
		Headers(header...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colMauve)).Padding(0, 1)
			}
			return lipgloss.NewStyle().Foreground(lipgloss.Color(colText)).Padding(0, 1)
		})
	// Do NOT call .Width(): let the table take its natural width so wide tables
	// overflow the pane and scroll horizontally.
	for _, ln := range strings.Split(tbl.String(), "\n") {
		if ln == "" {
			continue
		}
		r.lines = append(r.lines, Line{Text: ln, Wide: true})
	}
}

// bandCode wraps one fg-highlighted code line in a continuous background that
// survives chroma's per-token "\x1b[0m" resets, padded to `width` visible
// columns. chroma sets only foreground (we removed its Background); the bg is
// ours and is re-applied after each reset so it never drops mid-line.
func bandCode(line string, width int) string {
	s := codeBgANSI + strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+codeBgANSI)
	if pad := width - lipgloss.Width(line); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s + "\x1b[0m"
}

// code renders a (fenced) code block: chroma-highlighted, NOT wrapped, each
// line padded to the target width with a continuous code background. Wide=true.
// If the block has a language, a right-aligned language label is emitted first
// (Wide=false).
func (r *renderer) code(n ast.Node) {
	var raw strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		raw.Write(seg.Value(r.src))
	}
	src := strings.TrimRight(raw.String(), "\n")

	lang := ""
	if fc, ok := n.(*ast.FencedCodeBlock); ok && fc.Info != nil {
		lang = string(fc.Info.Segment.Value(r.src))
		lang = strings.TrimSpace(lang)
		if sp := strings.IndexByte(lang, ' '); sp >= 0 {
			lang = lang[:sp]
		}
	}
	width := r.width // content width

	// Language label: right-aligned, 1 space from the right edge.
	if lang != "" {
		label := lang + " "
		lead := width - lipgloss.Width(label)
		if lead < 0 {
			lead = 0
		}
		styled := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay1)).Render(lang) + " "
		r.lines = append(r.lines, Line{Text: strings.Repeat(" ", lead) + styled, Wide: false})
	}

	highlighted := highlight(src, lang)
	rawLines := strings.Split(src, "\n")
	hlLines := strings.Split(highlighted, "\n")

	maxw := 0
	for _, l := range rawLines {
		if w := lipgloss.Width(l); w > maxw {
			maxw = w
		}
	}
	// target visible width of each banded line; +2 for the 1-col inset each side.
	target := maxw + 2
	if target < width {
		target = width
	}
	for _, hl := range hlLines {
		r.lines = append(r.lines, Line{Text: bandCode(" "+hl, target), Wide: true})
	}
}

// highlight runs chroma over src; on any failure it returns src unchanged.
func highlight(src, lang string) string {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Analyse(src)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	it, err := lexer.Tokenise(nil, src)
	if err != nil {
		return src
	}
	f := formatters.Get("terminal16m")
	if f == nil {
		return src
	}
	var buf bytes.Buffer
	if err := f.Format(&buf, codeStyle(), it); err != nil {
		return src
	}
	return strings.TrimRight(buf.String(), "\n")
}

// quote renders a block quote: full-width band, "│ " indent token, prose wraps.
func (r *renderer) quote(n ast.Node, indent int) {
	start := len(r.lines)
	r.block(n, indent) // render children as normal prose first
	token := lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0)).Background(lipgloss.Color(colMantle)).Render("│ ")
	for i := start; i < len(r.lines); i++ {
		content := r.lines[i].Text
		// Pad to full pane width so the band spans the line.
		w := r.width - lipgloss.Width(token)
		if w < 1 {
			w = 1
		}
		padded := lipgloss.NewStyle().Width(w).Background(lipgloss.Color(colMantle)).Foreground(lipgloss.Color(colText)).Render(strip(content))
		r.lines[i] = Line{Text: token + padded, Wide: false}
	}
}
