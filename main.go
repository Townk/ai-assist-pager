package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
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

	prog := tea.NewProgram(
		newModel(harness, md),
		tea.WithInput(tty),
		tea.WithOutput(tty),
	)
	final, err := prog.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ai-assist-pager: %v\n", err)
		os.Exit(1)
	}
	// Alt-screen is torn down on exit; print the static render so the pane parks
	// with the reply visible instead of a blank pane.
	if fm, ok := final.(model); ok {
		fmt.Print(fm.staticRender())
	}
}
