package ui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/gdamore/tcell/v2"
)

// bashSegments preserves source exactly; malformed or oversized input stays plain.
func bashSegments(source string) (segments []Seg) {
	defer func() {
		if recover() != nil {
			segments = []Seg{{Text: source, FG: FgWhite}}
		}
	}()
	if len(source) > 256<<10 {
		return []Seg{{Text: source, FG: FgWhite}}
	}
	tokens, found := cachedBashTokens(source)
	if !found {
		lexer := lexers.Get("bash")
		it, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root", EnsureLF: false}, source+"\n")
		if err != nil {
			return []Seg{{Text: source, FG: FgWhite}}
		}
		tokens = it.Tokens()
		for i := len(tokens) - 1; i >= 0; i-- {
			if tokens[i].Value != "" {
				tokens[i].Value = strings.TrimSuffix(tokens[i].Value, "\n")
				break
			}
		}
		var text strings.Builder
		for _, token := range tokens {
			text.WriteString(token.Value)
		}
		if text.String() != source {
			return []Seg{{Text: source, FG: FgWhite}}
		}
		cacheBashTokens(source, tokens)
	}
	for _, token := range tokens {
		segments = append(segments, Seg{Text: token.Value, FG: bashTokenColor(token.Type)})
	}
	return segments
}

func bashTokenColor(t chroma.TokenType) tcell.Color {
	switch {
	case t.InCategory(chroma.Comment):
		return FgDim
	case t.InCategory(chroma.Keyword):
		return Purple
	case t.InSubCategory(chroma.NameVariable):
		return Orange
	case t.InSubCategory(chroma.LiteralString):
		return Green
	case t.InSubCategory(chroma.LiteralNumber):
		return Orange
	case t == chroma.NameBuiltin || t == chroma.NameFunction:
		return Blue
	case t.InCategory(chroma.Operator) || t.InCategory(chroma.Punctuation):
		return FgSoft
	default:
		return FgWhite
	}
}
