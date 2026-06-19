package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
	var fifoPath string
	flag.StringVar(&fifoPath, "actions-fifo", "", "FIFO path to write button actions to")
	var inputFifo string
	flag.StringVar(&inputFifo, "input-fifo", "", "FIFO path to read the input stream from (else stdin)")
	var thinkingLabel string
	flag.StringVar(&thinkingLabel, "thinking-label", "Working…", "default spinner label")
	flag.Parse()

	// Input source: the named FIFO (opens for read; blocks until a writer
	// connects) or stdin. Content streams in; keys come from /dev/tty.
	var src io.Reader = os.Stdin
	if inputFifo != "" {
		f, err := os.OpenFile(inputFifo, os.O_RDONLY, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ai-assist-pager: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		src = f
	}
	parser := &streamParser{}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		// No TTY (tests / pipes): drain the stream, strip control records, render
		// once, and exit.
		var b strings.Builder
		buf := make([]byte, 4096)
		rd := bufio.NewReader(src)
		for {
			n, rerr := rd.Read(buf)
			for _, ev := range parser.feed(buf[:n]) {
				if te, ok := ev.(textEvent); ok {
					b.WriteString(te.text)
				}
			}
			if rerr != nil {
				break
			}
		}
		m := newModel(harness, b.String())
		m.width = 100
		m.fifoPath = fifoPath
		fmt.Print(m.staticRender())
		return
	}
	defer tty.Close()

	// Force TrueColor: zellij's alt-screen pane underreports the color profile
	// during bubbletea's auto-detection, causing colors to be downsampled.
	// The UI targets a truecolor Catppuccin terminal, so we pin it explicitly.
	m := newModel(harness, "")
	m.fifoPath = fifoPath
	m.defaultLabel = thinkingLabel
	m.thinkLabel = thinkingLabel
	m.thinking = true // implicit thinking at launch (spec)
	m.streaming = true
	m.reader = bufio.NewReader(src)
	m.parser = parser
	prog := tea.NewProgram(
		m,
		tea.WithInput(tty),
		tea.WithOutput(tty),
		tea.WithColorProfile(colorprofile.TrueColor),
	)
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ai-assist-pager: %v\n", err)
		os.Exit(1)
	}
}
