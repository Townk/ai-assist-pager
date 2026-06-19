package main

import (
	"bufio"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitActionWritesFifoLine(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "act") // a regular file works for append+read in the test
	m := model{fifoPath: fifo}
	m.emitAction(Button{Kind: "copy", Payload: "echo hi\nls"})
	f, err := os.Open(fifo)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	line, _ := bufio.NewReader(f).ReadString('\n')
	line = strings.TrimRight(line, "\n")
	parts := strings.SplitN(line, "\t", 2)
	if parts[0] != "copy" {
		t.Fatalf("kind = %q", parts[0])
	}
	dec, _ := base64.StdEncoding.DecodeString(parts[1])
	if string(dec) != "echo hi\nls" {
		t.Fatalf("payload decoded = %q", string(dec))
	}
}

func TestEmitActionNoFifoIsNoop(t *testing.T) {
	m := model{fifoPath: ""}
	m.emitAction(Button{Kind: "copy", Payload: "x"}) // must not panic
}
