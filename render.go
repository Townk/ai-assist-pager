package main

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// strip removes ANSI SGR sequences so callers can measure/compare plain text.
func strip(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if c == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteByte(c)
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
		default:
			// Code blocks / quotes / tables are added in later tasks. Until then
			// fall back to inline text so nothing is silently dropped.
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
			st := lipgloss.NewStyle().Foreground(lipgloss.Color(colPeach)).Background(lipgloss.Color(colMantle))
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
