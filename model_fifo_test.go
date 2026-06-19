package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitActionWritesFifoLine(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "act")
	m := model{fifoPath: fifo}
	m.emitAction(Button{Kind: "copy", Payload: "echo hi\nls"})
	f, err := os.Open(fifo)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rec, _ := bufio.NewReader(f).ReadString('\x1e')
	rec = strings.TrimSuffix(rec, "\x1e")
	kind, payload, ok := strings.Cut(rec, "\x1f")
	if !ok || kind != "copy" {
		t.Fatalf("kind = %q ok = %v", kind, ok)
	}
	if payload != "echo hi\nls" {
		t.Fatalf("payload = %q, want %q", payload, "echo hi\nls")
	}
}

func TestEmitActionNoFifoIsNoop(t *testing.T) {
	m := model{fifoPath: ""}
	m.emitAction(Button{Kind: "copy", Payload: "x"}) // must not panic
}
