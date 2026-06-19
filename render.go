package main

import (
	"bytes"
	"regexp"
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

// admon describes one admonition type: display title, nerd-font icon glyph, and
// palette color. The icon field is easy to tweak per-glyph if the user's font
// doesn't include a particular codepoint.
type admon struct {
	title string
	icon  string // nerd-font glyph; swap codepoint here if it renders as tofu
	color string // Catppuccin hex constant
}

var admonitions = map[string]admon{
	"note":      {"Note", "󰋽", colBlue},
	"tip":       {"Tip", "󰌶", colGreen},
	"important": {"Important", "󰀦", colMauve},
	"warning":   {"Warning", "󰀪", colPeach},
	"caution":   {"Caution", "󰳦", colRed},
	"quote":     {"Quote", "󱆨", colOverlay0},
}

// admonMarkerRe matches an optional leading [!TYPE] marker in a block-quote body.
var admonMarkerRe = regexp.MustCompile(`(?is)^\s*\[!(\w+)\]\s*`)

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
	src     []byte
	width   int
	lines   []Line
	buttons []Button
}

// Render parses markdown and returns tagged, laid-out lines and a button
// registry for a given pane width. The button slice is empty until a later
// task populates it; callers should discard it with _ if unused.
func Render(md string, width int) ([]Line, []Button) {
	if width < 1 {
		width = 1
	}
	src := []byte(md)
	gm := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := gm.Parser().Parse(text.NewReader(src))
	r := &renderer{src: src, width: width}
	r.block(doc, 0)
	return r.lines, r.buttons
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
	// An HBar row has empty Text but is NOT blank — it's a horizontal scrollbar
	// the View draws dynamically. Keep it even when it ends the document (an
	// overflowing code block as the last element), or its scrollbar is lost.
	for len(r.lines) > 0 {
		last := r.lines[len(r.lines)-1]
		if last.HBar > 0 || strings.TrimSpace(strip(last.Text)) != "" {
			break
		}
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

// band wraps content in a continuous background sequence `bg` that survives
// embedded "\x1b[0m"/"\x1b[m" resets, padded to `width` visible columns.
// The bg sequence is re-applied after every reset so it never drops mid-line.
func band(content string, bg string, width int) string {
	s := bg + strings.ReplaceAll(content, "\x1b[0m", "\x1b[0m"+bg)
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+bg)
	if pad := width - lipgloss.Width(content); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s + "\x1b[0m"
}

// isShellLang reports whether a fenced-code language is a shell the run button
// should appear for. Resolves aliases (sh/bash/zsh/console/shell-session → shell).
func isShellLang(lang string) bool {
	key := strings.ToLower(strings.TrimSpace(lang))
	if canon, ok := langAliases[key]; ok {
		key = canon
	}
	return key == "shell"
}

const (
	glyphSep  = "❘"          // U+2758 buttons separator
	glyphPlay = "⏵"          // U+23F5 run
	glyphCopy = "\U0010F0C5" // copy
)

// code renders a (fenced) code block: chroma-highlighted, NOT wrapped, each
// line padded to the target width with a continuous code background. Wide=true.
// A decorative tab line (Wide=false) is emitted first: <leading pad><lang><" ❘ "><run? ><copy>.
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

	// Decorative tab: <leading pad><lang><" ❘ "><run? ><copy>. Each cell on the
	// code bg. Buttons (run/copy) are the 2-cell <glyph>" " units; record their
	// columns for mouse/keyboard activation.
	lineIdx := len(r.lines)
	bg := lipgloss.NewStyle().Background(lipgloss.Color(colCodeBg))

	// Lang part (icon or text label) and its display width.
	var langPart string
	var langW int
	if glyph, color, ok := langIcon(lang); ok && lang != "" {
		langPart = bg.Foreground(lipgloss.Color(color)).Render(glyph + " " + lang)
		langW = lipgloss.Width(glyph) + 1 + lipgloss.Width(lang)
	} else if lang != "" {
		langPart = bg.Foreground(lipgloss.Color(colOverlay1)).Render(lang)
		langW = lipgloss.Width(lang)
	} else {
		langPart = "" // no language: no lang part, but separator+buttons still show
		langW = 0
	}

	shell := isShellLang(lang)
	// region width: leadpad(1) + langW + sep(" ❘ "=3) + run(2 if shell) + copy(2)
	regionW := 1 + langW + 3 + 2
	if shell {
		regionW += 2
	}
	fillCols := width - regionW
	if fillCols < 0 {
		fillCols = 0
	}

	var sb strings.Builder
	sb.WriteString(codeFgANSI + strings.Repeat("▂", fillCols) + "\x1b[0m")
	col := fillCols

	sb.WriteString(bg.Render(" "))
	col++ // leading pad
	if langPart != "" {
		sb.WriteString(langPart)
		col += langW
	}
	// separator " ❘ "
	sb.WriteString(bg.Render(" "))
	col++
	sb.WriteString(bg.Foreground(lipgloss.Color(colOverlay0)).Render(glyphSep))
	col++
	sb.WriteString(bg.Render(" "))
	col++
	if shell {
		runCol := col
		sb.WriteString(bg.Foreground(lipgloss.Color(colGreen)).Render(glyphPlay))
		col++
		sb.WriteString(bg.Render(" "))
		col++
		r.buttons = append(r.buttons, Button{Line: lineIdx, Col: runCol, Width: 2, Kind: "play", Payload: src})
	}
	copyCol := col
	sb.WriteString(bg.Foreground(lipgloss.Color(colYellow)).Render(glyphCopy))
	col++
	sb.WriteString(bg.Render(" "))
	col++
	r.buttons = append(r.buttons, Button{Line: lineIdx, Col: copyCol, Width: 2, Kind: "copy", Payload: src})

	r.lines = append(r.lines, Line{Text: sb.String(), Wide: false, Code: true})

	highlighted := highlight(src, lang)
	hlLines := strings.Split(highlighted, "\n")
	blockW := 0
	for _, hl := range hlLines {
		body := " " + hl + " "
		r.lines = append(r.lines, Line{Text: body, Wide: true, Bg: codeBgANSI, Code: true})
		if w := lipgloss.Width(body); w > blockW {
			blockW = w
		}
	}

	// When the block overflows the viewport, the horizontal scrollbar row caps
	// it (and reads as the bottom padding) — so we skip the 🮂 bottom bar there,
	// which otherwise looks redundant/unpolished. Non-overflowing blocks keep
	// the normal 🮂 bottom edge.
	if blockW > width {
		r.lines = append(r.lines, Line{Wide: false, HBar: blockW, Code: true})
	} else {
		// Bottom edge bar: 🮂 characters in fg colCodeBg (#282C41), no background.
		// Total display width == width. Wide=false.
		bottomLine := codeFgANSI + strings.Repeat("🮂", width) + "\x1b[0m"
		r.lines = append(r.lines, Line{Text: bottomLine, Wide: false, Code: true})
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

// quote renders a block quote as a GitHub-style admonition.
// It collects the child block text, optionally detects a [!TYPE] marker, then
// emits a colored ▋ border on every line. A recognized [!TYPE] marker also
// emits a header line with icon + title. A bare quote (no marker) emits only
// the bordered body lines (no header) with a colOverlay0 border.
func (r *renderer) quote(n ast.Node, indent int) {
	// Step 1: collect body text from child blocks.
	var pieces []string
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.(type) {
		case *ast.Paragraph, *ast.TextBlock:
			pieces = append(pieces, r.inline(c))
		default:
			if t := strings.TrimSpace(r.inline(c)); t != "" {
				pieces = append(pieces, t)
			}
		}
	}
	body := strings.Join(pieces, "\n")

	// Step 2: detect [!type] marker.
	var a *admon
	if m := admonMarkerRe.FindStringSubmatch(body); m != nil {
		key := strings.ToLower(m[1])
		if entry, ok := admonitions[key]; ok {
			a = &entry
			body = admonMarkerRe.ReplaceAllString(body, "")
		}
	}

	// Step 3: determine border color and dark background.
	color := colOverlay0
	if a != nil {
		color = a.color
	}
	bg := bgANSI(darken(color, 0.20))

	// Step 4: build styles.
	borderGlyph := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("▋")
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colText)).Italic(true)

	// Step 5: emit header (only for recognized [!type] admonitions).
	if a != nil {
		headerText := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(a.icon + " " + a.title)
		inner := borderGlyph + " " + headerText
		r.lines = append(r.lines, Line{Text: band(inner, bg, r.width), Wide: false})
	}

	// Step 6: emit body lines. Wrap to width-3: border (1) + leading space (1) +
	// text + a reserved trailing column so the background always pads at least one
	// space past the text on the right (no text touching the band's right edge).
	trimmed := strings.TrimSpace(body)
	if trimmed != "" {
		w := r.width - 3
		if w < 1 {
			w = 1
		}
		wrapped := bodyStyle.Width(w).Render(trimmed)
		for _, ln := range strings.Split(wrapped, "\n") {
			inner := borderGlyph + " " + ln
			r.lines = append(r.lines, Line{Text: band(inner, bg, r.width), Wide: false})
		}
	}
}
