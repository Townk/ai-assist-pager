package main

// Button is one activatable control on a code-block tab. Line indexes into the
// []Line returned by Render; Col/Width are the glyph+trailing-space click target
// within that line's content (before the model's 2-col left margin). Kind is
// "play" or "copy"; Payload is the code block's raw source.
type Button struct {
	Line    int
	Col     int
	Width   int
	Kind    string
	Payload string
}

// hintAlphabet is the ordered set of single-char labels used by assignHintLabels.
const hintAlphabet = "asdfghjklqwertyuiop"

// buttonAt maps a mouse click at screen position (x, y) to a Button.
// bodyTop is the screen row of the first body line; yOff is the viewport offset.
// The content column is x-2 (2-col left margin); the line index is yOff+(y-bodyTop).
func buttonAt(buttons []Button, x, y, yOff, bodyTop int) (Button, bool) {
	if y < bodyTop {
		return Button{}, false
	}
	line := yOff + (y - bodyTop)
	col := x - 2 // 2-col left margin
	for _, b := range buttons {
		if b.Line == line && col >= b.Col && col < b.Col+b.Width {
			return b, true
		}
	}
	return Button{}, false
}

// assignHintLabels assigns distinct single-char labels from hintAlphabet to
// each visible button in order, returning a map from label to Button.
func assignHintLabels(visible []Button) map[string]Button {
	out := make(map[string]Button, len(visible))
	for i, b := range visible {
		if i >= len(hintAlphabet) {
			break
		}
		out[string(hintAlphabet[i])] = b
	}
	return out
}
