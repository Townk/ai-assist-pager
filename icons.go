package main

import "strings"

type langIconDef struct {
	glyph string // nerd-font glyph
	color string // "#RRGGBB" — the language's signature color
}

// langIcons maps a CANONICAL language name to its icon. Glyphs/colors are seeded
// from nvim-web-devicons; treat them as tweakable (swap a codepoint/color in one
// place if your font renders it differently). Extend freely.
var langIcons = map[string]langIconDef{
	"go":         {"", "#00ADD8"},
	"python":     {"", "#FFBC03"},
	"javascript": {"", "#F1E05A"},
	"typescript": {"", "#3178C6"},
	"rust":       {"", "#DEA584"},
	"shell":      {"", "#89E051"},
	"c":          {"", "#599EFF"},
	"cpp":        {"", "#F34B7D"},
	"java":       {"", "#CC3E44"},
	"ruby":       {"", "#CC342D"},
	"php":        {"", "#A074C4"},
	"html":       {"", "#E44D26"},
	"css":        {"", "#42A5F5"},
	"json":       {"", "#CBCB41"},
	"yaml":       {"", "#6D8086"},
	"toml":       {"", "#9C4221"},
	"markdown":   {"", "#519ABA"},
	"sql":        {"", "#DAD8D8"},
	"lua":        {"", "#51A0CF"},
	"dockerfile": {"", "#099CEC"},
	"make":       {"", "#6D8086"},
	"nix":        {"", "#7EBAE4"},
	"kotlin":     {"", "#7F52FF"},
	"swift":      {"", "#F05138"},
	"xml":        {"󰗀", "#E37933"},
	"ini":        {"󰒓", "#6D8086"},
	"plantuml":   {"", "#D6A85B"},
	"mermaid":    {"􎚕", "#FF3670"},
}

// langAliases maps the strings authors actually write in a fence to the
// canonical key above.
var langAliases = map[string]string{
	"py": "python", "js": "javascript", "ts": "typescript",
	"rs": "rust", "sh": "shell", "bash": "shell", "zsh": "shell",
	"console": "shell", "shell-session": "shell", "c++": "cpp", "cxx": "cpp",
	"rb": "ruby", "md": "markdown", "yml": "yaml", "kt": "kotlin",
	"docker": "dockerfile", "makefile": "make",
	"puml": "plantuml", "uml": "plantuml", "mmd": "mermaid",
	"conf": "ini", "cfg": "ini", "config": "ini", "editorconfig": "ini",
}

// langIcon returns the glyph + color for a fenced-code language. ok is false
// when there's no icon (caller falls back to the text label). Matching is
// case-insensitive and alias-aware.
func langIcon(lang string) (glyph string, color string, ok bool) {
	key := strings.ToLower(strings.TrimSpace(lang))
	if canon, isAlias := langAliases[key]; isAlias {
		key = canon
	}
	if def, found := langIcons[key]; found {
		return def.glyph, def.color, true
	}
	return "", "", false
}
