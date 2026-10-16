package ui

import (
	"regexp"
	"strings"
)

var jsTokens = regexp.MustCompile(`"[^"]*"|'[^']*'|\b(?:const|let|var|await|async|return|for|of|if|else|function|new|throw|typeof|import|export)\b|\b\d+\b|\s+|.`)

var jsKeywords = map[string]bool{
	"const": true, "let": true, "var": true, "await": true, "async": true, "return": true,
	"for": true, "of": true, "if": true, "else": true, "function": true, "new": true,
	"throw": true, "typeof": true, "import": true, "export": true,
}

func HighlightJS(line string) []Seg {
	var out []Seg
	for _, tok := range jsTokens.FindAllString(line, -1) {
		fg := FgWhite
		switch {
		case strings.HasPrefix(tok, `"`) || strings.HasPrefix(tok, `'`):
			fg = Green
		case jsKeywords[tok]:
			fg = Purple
		case isNumber(tok):
			fg = Orange
		}
		out = append(out, Seg{Text: tok, FG: fg})
	}
	return out
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
