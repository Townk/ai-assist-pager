package main

import "testing"

func TestLangIcon(t *testing.T) {
	// Known canonical language: go.
	t.Run("known_go", func(t *testing.T) {
		glyph, color, ok := langIcon("go")
		if !ok {
			t.Fatal("langIcon(\"go\") ok=false, want true")
		}
		want := langIcons["go"]
		if glyph != want.glyph {
			t.Fatalf("glyph = %q, want %q", glyph, want.glyph)
		}
		if color != want.color {
			t.Fatalf("color = %q, want %q", color, want.color)
		}
	})

	// Alias: py → python.
	t.Run("alias_py", func(t *testing.T) {
		glyph, color, ok := langIcon("py")
		if !ok {
			t.Fatal("langIcon(\"py\") ok=false, want true")
		}
		want := langIcons["python"]
		if glyph != want.glyph {
			t.Fatalf("py glyph = %q, want python glyph %q", glyph, want.glyph)
		}
		if color != want.color {
			t.Fatalf("py color = %q, want python color %q", color, want.color)
		}
	})

	// Alias: bash → shell.
	t.Run("alias_bash", func(t *testing.T) {
		glyph, color, ok := langIcon("bash")
		if !ok {
			t.Fatal("langIcon(\"bash\") ok=false, want true")
		}
		want := langIcons["shell"]
		if glyph != want.glyph {
			t.Fatalf("bash glyph = %q, want shell glyph %q", glyph, want.glyph)
		}
		if color != want.color {
			t.Fatalf("bash color = %q, want shell color %q", color, want.color)
		}
	})

	// Unknown language: ok must be false.
	t.Run("unknown_brainfuck", func(t *testing.T) {
		glyph, _, ok := langIcon("brainfuck")
		if ok {
			t.Fatal("langIcon(\"brainfuck\") ok=true, want false")
		}
		if glyph != "" {
			t.Fatalf("unknown glyph = %q, want empty", glyph)
		}
	})

	// Empty string: ok must be false.
	t.Run("empty", func(t *testing.T) {
		glyph, _, ok := langIcon("")
		if ok {
			t.Fatal("langIcon(\"\") ok=true, want false")
		}
		if glyph != "" {
			t.Fatalf("empty glyph = %q, want empty", glyph)
		}
	})

	// Case-insensitive: Go → go icon.
	t.Run("case_Go", func(t *testing.T) {
		glyph, _, ok := langIcon("Go")
		if !ok {
			t.Fatal("langIcon(\"Go\") ok=false, want true")
		}
		if glyph != langIcons["go"].glyph {
			t.Fatalf("Go glyph = %q, want %q", glyph, langIcons["go"].glyph)
		}
	})

	// Case-insensitive: PYTHON → python icon.
	t.Run("case_PYTHON", func(t *testing.T) {
		glyph, _, ok := langIcon("PYTHON")
		if !ok {
			t.Fatal("langIcon(\"PYTHON\") ok=false, want true")
		}
		if glyph != langIcons["python"].glyph {
			t.Fatalf("PYTHON glyph = %q, want %q", glyph, langIcons["python"].glyph)
		}
	})
}
