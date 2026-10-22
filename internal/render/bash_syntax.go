package render

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
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
	segments = bashColorize(tokens)
	return segments
}
