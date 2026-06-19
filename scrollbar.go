package main

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

// vthumb returns the thumb [pos, pos+size) within a `visible`-row track for a
// `total`-row document scrolled to `off`. size≥1; (0,0) when the content fits.
func vthumb(total, visible, off int) (pos, size int) {
	if total <= visible || visible < 1 {
		return 0, 0
	}
	size = visible * visible / total
	if size < 1 {
		size = 1
	}
	pos = (visible - size) * off / (total - visible)
	return pos, size
}

// hthumb returns the thumb [pos, pos+size) within a `view`-wide track for a
// `blockW`-column block scrolled to `xoff` (clamped). Full track when blockW≤view.
func hthumb(blockW, view, xoff int) (pos, size int) {
	if view < 1 {
		view = 1
	}
	if blockW <= view {
		return 0, view
	}
	size = view * view / blockW
	if size < 1 {
		size = 1
	}
	maxX := blockW - view
	if xoff < 0 {
		xoff = 0
	} else if xoff > maxX {
		xoff = maxX
	}
	pos = (view - size) * xoff / maxX
	return pos, size
}

// hscrollbarRow renders a cw-wide horizontal scrollbar (─ track / ━ thumb) on
// the code background at the block's current horizontal offset.
func hscrollbarRow(blockW, xoff, cw int) string {
	pos, size := hthumb(blockW, cw, xoff)
	if pos+size > cw {
		size = cw - pos
	}
	track := lipgloss.NewStyle().Background(lipgloss.Color(colCodeBg)).Foreground(lipgloss.Color(colSurface0))
	thumb := lipgloss.NewStyle().Background(lipgloss.Color(colCodeBg)).Foreground(lipgloss.Color(colOverlay1))
	var sb strings.Builder
	if pos > 0 {
		sb.WriteString(track.Render(strings.Repeat("─", pos)))
	}
	if size > 0 {
		sb.WriteString(thumb.Render(strings.Repeat("━", size)))
	}
	if tail := cw - pos - size; tail > 0 {
		sb.WriteString(track.Render(strings.Repeat("─", tail)))
	}
	return sb.String()
}

// vscrollCell returns the right-edge vertical-scrollbar cell (with a leading
// gap) for body row i, or "" when there is no scrollbar (size≤0).
func vscrollCell(i, pos, size int) string {
	if size <= 0 {
		return ""
	}
	if i >= pos && i < pos+size {
		return " " + lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay1)).Render("┃")
	}
	return " " + lipgloss.NewStyle().Foreground(lipgloss.Color(colSurface0)).Render("│")
}

// padTo right-pads s with spaces to w display columns (never truncates).
func padTo(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// hintCodeRow paints the row's visible text, muted, on a solid code-bg fill —
// the hint-mode look for code rows (keeps the block cohesive, no seams).
func hintCodeRow(row string, width int) string {
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(colSubtext))
	return band(muted.Render(strip(row)), codeBgANSI, width)
}

// overlayLabels splices each label char into an already-styled row at its
// display column (ANSI-aware, via hslice). Works on dim prose or filled code rows.
func overlayLabels(row string, labels map[int]string, lab lipgloss.Style) string {
	if len(labels) == 0 {
		return row
	}
	cols := make([]int, 0, len(labels))
	for c := range labels {
		cols = append(cols, c)
	}
	sort.Ints(cols)
	const big = 1 << 30
	var sb strings.Builder
	prev := 0
	for _, c := range cols {
		if c < prev {
			continue
		}
		sb.WriteString(hslice(row, prev, c-prev))
		sb.WriteString(lab.Render(labels[c]))
		prev = c + 1
	}
	sb.WriteString(hslice(row, prev, big))
	return sb.String()
}
