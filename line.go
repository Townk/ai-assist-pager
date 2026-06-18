package main

// Line is one rendered terminal line. Wide marks code-block / table lines that
// keep their natural width (may exceed the pane) and scroll horizontally; prose
// lines are pre-wrapped to the pane width and stay anchored (Wide=false).
type Line struct {
	Text string // styled (ANSI), ready to print
	Wide bool
}
