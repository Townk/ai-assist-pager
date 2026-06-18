package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/mattn/go-runewidth"
)

func main() {
	// Force narrow (1-cell) accounting for East-Asian-ambiguous characters
	// (em-dash, ellipsis, smart quotes, nerd-font icons).  The terminal renders
	// them as 1 cell; without this setting go-runewidth counts them as 2,
	// causing admonition/code background fills to come up short.
	// Must run before any lipgloss/bubbletea call: charmbracelet/x/ansi reads
	// RUNEWIDTH_EASTASIAN in its package init, so the env var must be set first.
	os.Setenv("RUNEWIDTH_EASTASIAN", "0")
	runewidth.DefaultCondition.EastAsianWidth = false

	var harness string
	flag.StringVar(&harness, "harness", "agent", "harness label for the header")
	flag.Parse()

	var md string
	if flag.NArg() >= 1 {
		b, err := os.ReadFile(flag.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "ai-assist-pager: %v\n", err)
			os.Exit(1)
		}
		md = string(b)
	} else {
		b, err := os.ReadFile("/dev/stdin")
		if err != nil {
			fmt.Fprintf(os.Stderr, "ai-assist-pager: %v\n", err)
			os.Exit(1)
		}
		md = string(b)
	}

	// Interact on /dev/tty so the file arg (content) and any stdin redirection
	// don't interfere with key input — the ai-assist-input lesson.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		// No TTY (tests / pipes): just print the static render and exit.
		m := newModel(harness, md)
		m.width = 100
		fmt.Print(m.staticRender())
		return
	}
	defer tty.Close()

	// Force TrueColor: zellij's alt-screen pane underreports the color profile
	// during bubbletea's auto-detection, causing colors to be downsampled.
	// The UI targets a truecolor Catppuccin terminal, so we pin it explicitly.
	prog := tea.NewProgram(
		newModel(harness, md),
		tea.WithInput(tty),
		tea.WithOutput(tty),
		tea.WithColorProfile(colorprofile.TrueColor),
	)
	// On quit (q/Esc) we exit straight away; the docked pane is spawned with
	// --close-on-exit, so it closes rather than parking. No static dump (it would
	// just flash before the pane closes).
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ai-assist-pager: %v\n", err)
		os.Exit(1)
	}
}
