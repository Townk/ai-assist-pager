package main

import "testing"

func collect(p *streamParser, chunks ...string) []streamEvent {
	var all []streamEvent
	for _, c := range chunks {
		all = append(all, p.feed([]byte(c))...)
	}
	return all
}

func TestParserPlainText(t *testing.T) {
	got := collect(&streamParser{}, "# Hello\nworld")
	if len(got) != 1 {
		t.Fatalf("want 1 event, got %d (%#v)", len(got), got)
	}
	te, ok := got[0].(textEvent)
	if !ok || te.text != "# Hello\nworld" {
		t.Fatalf("want textEvent %q, got %#v", "# Hello\nworld", got[0])
	}
}

func TestParserThinkWithLabel(t *testing.T) {
	// "ab" + DLE t "Reading…" DLE + "cd"
	got := collect(&streamParser{}, "ab\x10tReading…\x10cd")
	if len(got) != 3 {
		t.Fatalf("want 3 events, got %d (%#v)", len(got), got)
	}
	if te, ok := got[0].(textEvent); !ok || te.text != "ab" {
		t.Fatalf("event0 want text ab, got %#v", got[0])
	}
	if th, ok := got[1].(thinkEvent); !ok || th.label != "Reading…" {
		t.Fatalf("event1 want think Reading…, got %#v", got[1])
	}
	if te, ok := got[2].(textEvent); !ok || te.text != "cd" {
		t.Fatalf("event2 want text cd, got %#v", got[2])
	}
}

func TestParserThinkNoLabel(t *testing.T) {
	got := collect(&streamParser{}, "\x10t\x10")
	if len(got) != 1 {
		t.Fatalf("want 1 event, got %d (%#v)", len(got), got)
	}
	if th, ok := got[0].(thinkEvent); !ok || th.label != "" {
		t.Fatalf("want empty-label thinkEvent, got %#v", got[0])
	}
}

func TestParserRecordSplitAcrossChunks(t *testing.T) {
	// The DLE record is split mid-label across three feeds.
	got := collect(&streamParser{}, "x\x10tSear", "ching", "…\x10y")
	if len(got) != 3 {
		t.Fatalf("want 3 events, got %d (%#v)", len(got), got)
	}
	if th, ok := got[1].(thinkEvent); !ok || th.label != "Searching…" {
		t.Fatalf("want reassembled label Searching…, got %#v", got[1])
	}
	if te, ok := got[2].(textEvent); !ok || te.text != "y" {
		t.Fatalf("want trailing text y, got %#v", got[2])
	}
}
