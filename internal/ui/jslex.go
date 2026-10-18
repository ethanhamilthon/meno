package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// jsKind classifies a lexical token of JavaScript source.
type jsKind int

const (
	jsSpace jsKind = iota
	jsIdent
	jsKeyword
	jsLiteral // true false null undefined NaN Infinity
	jsNumber
	jsString   // the whole of a quoted string, delimiters included
	jsTplOpen  // the opening backtick
	jsTplText  // literal text of a template
	jsTplExpr  // the dollar-brace that opens an interpolation, or the } that closes it
	jsTplClose // the closing backtick
	jsRegex
	jsComment
	jsPunct
)

type jsTok struct {
	Kind jsKind
	Text string
}

var jsKeywordSet = words("async await break case catch class const continue debugger default delete do else export extends finally for from function if import in instanceof let new of return static super switch this throw try typeof var void while with yield get set")
var jsLiteralSet = words("true false null undefined NaN Infinity")
var regexAfter = words("return typeof instanceof in of new delete void throw case do else yield await")

// lexJS splits JavaScript source into tokens that concatenate back to the
// source. ok is false for source it cannot tokenize safely (an unterminated
// string, template, comment or regular expression): the caller then shows the
// source unhighlighted instead of guessing.
func lexJS(src string) (toks []jsTok, ok bool) {
	l := &jsLexer{s: src}
	if !l.run(false) {
		return nil, false
	}
	return l.toks, true
}

type jsLexer struct {
	s    string
	i    int
	toks []jsTok
	prev jsTok // last significant token
	has  bool
}

func (l *jsLexer) emit(k jsKind, text string) {
	l.toks = append(l.toks, jsTok{k, text})
	if k != jsSpace && k != jsComment {
		l.prev, l.has = jsTok{k, text}, true
	}
}

func (l *jsLexer) regexAllowed() bool {
	if !l.has {
		return true
	}
	switch l.prev.Kind {
	case jsNumber, jsString, jsTplClose, jsRegex, jsLiteral, jsIdent:
		return false
	case jsKeyword:
		return regexAfter[l.prev.Text]
	case jsPunct:
		return l.prev.Text != ")" && l.prev.Text != "]"
	}
	return true
}

// run lexes until the end, or until the } that closes a template interpolation
// when inTpl is set (that brace is left for the caller).
func (l *jsLexer) run(inTpl bool) bool {
	depth := 0
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch {
		case c == '}' && inTpl && depth == 0:
			return true
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			j := l.i
			for j < len(l.s) && strings.IndexByte(" \t\n\r", l.s[j]) >= 0 {
				j++
			}
			l.emit(jsSpace, l.s[l.i:j])
			l.i = j
		case c == '/' && l.i+1 < len(l.s) && l.s[l.i+1] == '/':
			j := strings.IndexByte(l.s[l.i:], '\n')
			if j < 0 {
				j = len(l.s) - l.i
			}
			l.emit(jsComment, l.s[l.i:l.i+j])
			l.i += j
		case c == '/' && l.i+1 < len(l.s) && l.s[l.i+1] == '*':
			j := strings.Index(l.s[l.i+2:], "*/")
			if j < 0 {
				return false
			}
			l.emit(jsComment, l.s[l.i:l.i+j+4])
			l.i += j + 4
		case c == '"' || c == '\'':
			j, ok := scanQuoted(l.s, l.i)
			if !ok {
				return false
			}
			l.emit(jsString, l.s[l.i:j])
			l.i = j
		case c == '`':
			if !l.template() {
				return false
			}
		case c == '/' && l.regexAllowed():
			j, ok := scanRegex(l.s, l.i)
			if !ok {
				return false
			}
			l.emit(jsRegex, l.s[l.i:j])
			l.i = j
		case c >= '0' && c <= '9' || c == '.' && l.i+1 < len(l.s) && l.s[l.i+1] >= '0' && l.s[l.i+1] <= '9':
			j := l.i + 1
			for j < len(l.s) && (isIdentByte(l.s[j]) || l.s[j] == '.' || (l.s[j] == '+' || l.s[j] == '-') && (l.s[j-1] == 'e' || l.s[j-1] == 'E') && !strings.HasPrefix(l.s[l.i:], "0x")) {
				j++
			}
			l.emit(jsNumber, l.s[l.i:j])
			l.i = j
		default:
			r, size := utf8.DecodeRuneInString(l.s[l.i:])
			if r == '_' || r == '$' || unicode.IsLetter(r) {
				j := l.i + size
				for j < len(l.s) {
					r2, s2 := utf8.DecodeRuneInString(l.s[j:])
					if !(r2 == '_' || r2 == '$' || unicode.IsLetter(r2) || unicode.IsDigit(r2)) {
						break
					}
					j += s2
				}
				word := l.s[l.i:j]
				switch {
				case l.has && l.prev.Kind == jsPunct && l.prev.Text == ".":
					l.emit(jsIdent, word)
				case jsLiteralSet[word]:
					l.emit(jsLiteral, word)
				case jsKeywordSet[word]:
					l.emit(jsKeyword, word)
				default:
					l.emit(jsIdent, word)
				}
				l.i = j
				continue
			}
			if inTpl {
				if c == '{' {
					depth++
				} else if c == '}' {
					depth--
				}
			}
			l.emit(jsPunct, l.s[l.i:l.i+size])
			l.i += size
		}
	}
	return !inTpl
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '$' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// scanQuoted returns the end of the string literal starting at i. A backslash
// escapes the next character, including a newline (line continuation).
func scanQuoted(s string, i int) (int, bool) {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return j + 1, true
		case '\n':
			return 0, false
		}
	}
	return 0, false
}

func scanRegex(s string, i int) (int, bool) {
	inClass := false
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '\n':
			return 0, false
		case '/':
			if !inClass {
				j++
				for j < len(s) && isIdentByte(s[j]) {
					j++
				}
				return j, true
			}
		}
	}
	return 0, false
}

// template lexes a template literal: literal text, interpolations lexed as
// code, and the delimiters.
func (l *jsLexer) template() bool {
	l.emit(jsTplOpen, "`")
	l.i++
	start := l.i
	flush := func(end int) {
		if end > start {
			l.emit(jsTplText, l.s[start:end])
		}
	}
	for l.i < len(l.s) {
		switch {
		case l.s[l.i] == '\\':
			l.i += 2
		case l.s[l.i] == '`':
			flush(l.i)
			l.emit(jsTplClose, "`")
			l.i++
			return true
		case l.s[l.i] == '$' && l.i+1 < len(l.s) && l.s[l.i+1] == '{':
			flush(l.i)
			l.emit(jsTplExpr, "${")
			l.i += 2
			l.has = false
			if !l.run(true) || l.i >= len(l.s) {
				return false
			}
			l.emit(jsTplExpr, "}")
			l.i++
			start = l.i
		default:
			l.i++
		}
	}
	return false
}
