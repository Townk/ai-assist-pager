package main

import (
	"testing"
)

func TestWindowProseIgnoresXOffset(t *testing.T) {
	lines := []Line{{Text: "hello", Wide: false}}
	out := Window(lines, 3 /*xOffset*/, 0, 10, 1)
	if len(out) != 1 || out[0] != "hello" {
		t.Fatalf("prose should ignore xOffset, got %q", out)
	}
}

func TestWindowWideAppliesXOffset(t *testing.T) {
	lines := []Line{{Text: "0123456789", Wide: true}}
	out := Window(lines, 4 /*xOffset*/, 0, 3 /*width*/, 1)
	if len(out) != 1 || strip(out[0]) != "456" {
		t.Fatalf("wide line should show slice [4:7] = 456, got %q", strip(out[0]))
	}
}

func TestWindowVerticalOffsetAndHeightPadding(t *testing.T) {
	lines := []Line{{Text: "a"}, {Text: "b"}, {Text: "c"}}
	out := Window(lines, 0, 1 /*yOffset*/, 10, 4 /*height*/)
	if len(out) != 4 {
		t.Fatalf("expected 4 rows (padded), got %d", len(out))
	}
	if out[0] != "b" || out[1] != "c" || out[2] != "" || out[3] != "" {
		t.Fatalf("unexpected window: %#v", out)
	}
}

func TestMaxWideWidth(t *testing.T) {
	lines := []Line{{Text: "short", Wide: false}, {Text: "0123456789", Wide: true}, {Text: "012", Wide: true}}
	if got := MaxWideWidth(lines); got != 10 {
		t.Fatalf("MaxWideWidth = %d, want 10", got)
	}
	if got := MaxWideWidth([]Line{{Text: "p", Wide: false}}); got != 0 {
		t.Fatalf("MaxWideWidth with no wide lines = %d, want 0", got)
	}
}

