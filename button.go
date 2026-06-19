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
