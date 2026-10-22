package render

import (
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/v2"
	"github.com/gdamore/tcell/v2"
)

// bashSeparators end a command: the next word is a command name.
var bashSeparators = map[string]bool{";": true, "|": true, "||": true, "&&": true, "&": true, ";;": true, "|&": true}

// bashOpeners are keywords after which a command starts.
var bashOpeners = map[string]bool{"then": true, "do": true, "else": true, "elif": true, "if": true, "while": true, "until": true, "!": true, "{": true}

// bashColorize colors chroma tokens by role: the command name is the brightest,
// its arguments are softer, separators are faint. Text is never changed.
func bashColorize(tokens []chroma.Token) []Seg {
	var out []Seg
	add := func(text string, fg tcell.Color) {
		if text != "" {
			out = append(out, Seg{Text: text, FG: fg})
		}
	}
	expect, assign := true, false
	for i, tok := range tokens {
		v, t := tok.Value, tok.Type
		switch {
		case t.InCategory(chroma.Comment):
			add(v, FgDim)
		case bashSeparators[v]:
			add(v, FgFaint)
			expect, assign = true, false
		case strings.HasSuffix(v, "(") || strings.HasSuffix(v, "`") || v == "{":
			add(v, FgSoft)
			expect, assign = true, false
		case v == ")" || v == "}":
			add(v, FgSoft)
			expect = false
		case t == chroma.NameBuiltin || t == chroma.NameFunction:
			expect, assign = bashWords(v, expect, assign, add)
		case t.InCategory(chroma.Keyword):
			add(v, Purple)
			expect = bashOpeners[v]
		case t.InSubCategory(chroma.NameVariable):
			add(v, Orange)
			if expect && nextValue(tokens, i) == "=" {
				assign = true
			} else {
				expect = false
			}
		case t.InSubCategory(chroma.LiteralString):
			add(v, Green)
			if assign {
				assign = false
			} else {
				expect = false
			}
		case t.InSubCategory(chroma.LiteralNumber):
			add(v, Orange)
			if assign {
				assign = false
			} else {
				expect = false
			}
		case t.InCategory(chroma.Operator) || t.InCategory(chroma.Punctuation):
			add(v, FgSoft)
		default:
			expect, assign = bashWords(v, expect, assign, add)
		}
		if strings.Contains(v, "\n") && !t.InCategory(chroma.LiteralString) && !t.InCategory(chroma.Comment) {
			expect, assign = true, false
		}
	}
	return out
}

// bashWords splits a plain token into words and spaces. The first word where a
// command is expected is the command name; the value of VAR=value is skipped.
func bashWords(v string, expect, assign bool, add func(string, tcell.Color)) (bool, bool) {
	rest := v
	for rest != "" {
		n := 0
		space := unicode.IsSpace(rune(rest[0]))
		for n < len(rest) && unicode.IsSpace(rune(rest[n])) == space {
			n++
		}
		part := rest[:n]
		rest = rest[n:]
		switch {
		case space:
			add(part, FgSoft)
			if strings.Contains(part, "\n") {
				expect, assign = true, false
			} else if assign {
				assign = false
			}
		case assign:
			add(part, FgSoft)
			assign = false
		case expect:
			add(part, FgWhite)
			expect = false
		default:
			add(part, FgSoft)
		}
	}
	return expect, assign
}

// nextValue is the text of the next token that has any.
func nextValue(tokens []chroma.Token, i int) string {
	for j := i + 1; j < len(tokens); j++ {
		if tokens[j].Value != "" {
			return tokens[j].Value
		}
	}
	return ""
}
