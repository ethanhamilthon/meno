package render

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// codeLang describes just enough of a language to colour it: keywords, literals,
// comments, strings and numbers. It is a highlighter, not a parser.
type codeLang struct {
	keywords    map[string]bool
	literals    map[string]bool
	lineComment []string
	// hashNeedsSpace: '#' starts a comment only at a word start (shell).
	hashNeedsSpace bool
	blockStart     string
	blockEnd       string
	quotes         string   // single-line string delimiters
	raw            []string // multi-line string delimiters
	fold           bool     // keywords are case-insensitive (SQL)
	jsonKeys       bool     // "key": is a key
	yamlKeys       bool     // key: at the start of a line
	shell          bool
}

func words(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

var cLikeKeywords = words("if else for while do switch case break continue return static const struct class enum typedef void int char long short float double unsigned signed public private protected new this import package namespace using template typename include define final extends implements interface throws try catch finally throw var let func fun val abstract override virtual inline extern volatile sizeof default goto")

var codeLangs = map[string]*codeLang{
	"go": {
		keywords:    words("break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var"),
		literals:    words("true false nil iota"),
		lineComment: []string{"//"}, blockStart: "/*", blockEnd: "*/", quotes: "\"'", raw: []string{"`"},
	},
	"js": {
		keywords:    words("async await break case catch class const continue debugger default delete do else export extends finally for from function if import in instanceof let new of return static super switch this throw try typeof var void while yield interface type enum implements public private protected readonly as abstract declare namespace"),
		literals:    words("true false null undefined NaN Infinity"),
		lineComment: []string{"//"}, blockStart: "/*", blockEnd: "*/", quotes: "\"'", raw: []string{"`"},
	},
	"python": {
		keywords:    words("and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield match case"),
		literals:    words("True False None"),
		lineComment: []string{"#"}, quotes: "\"'", raw: []string{"\"\"\"", "'''"},
	},
	"rust": {
		keywords:    words("as async await break const continue crate dyn else enum extern fn for if impl in let loop match mod move mut pub ref return self Self static struct super trait type unsafe use where while"),
		literals:    words("true false"),
		lineComment: []string{"//"}, blockStart: "/*", blockEnd: "*/", quotes: "\"",
	},
	"clike": {
		keywords: cLikeKeywords, literals: words("true false null nullptr NULL nil"),
		lineComment: []string{"//"}, blockStart: "/*", blockEnd: "*/", quotes: "\"'",
	},
	"sql": {
		keywords:    words("select from where insert into values update set delete create table drop alter add column join left right inner outer full cross on group by order having limit offset and or not as distinct union all primary key foreign references index default unique check constraint begin commit rollback with case when then else end in is like between exists asc desc"),
		literals:    words("null true false"),
		lineComment: []string{"--"}, blockStart: "/*", blockEnd: "*/", quotes: "'\"", fold: true,
	},
	"json": {literals: words("true false null"), quotes: "\"", jsonKeys: true},
	"yaml": {literals: words("true false null yes no on off"), lineComment: []string{"#"}, hashNeedsSpace: true, quotes: "\"'", yamlKeys: true},
	"sh": {
		keywords:    words("if then else elif fi for while until do done case esac function in select return export local readonly set unset source alias"),
		literals:    words("true false"),
		lineComment: []string{"#"}, hashNeedsSpace: true, quotes: "\"'", shell: true,
	},
}

var langAliases = map[string]string{
	"golang": "go", "js": "js", "javascript": "js", "jsx": "js", "mjs": "js", "cjs": "js", "ts": "js", "typescript": "js", "tsx": "js", "json5": "json",
	"jsonc": "json", "py": "python", "python3": "python", "rs": "rust", "c": "clike", "h": "clike", "cpp": "clike", "c++": "clike", "cc": "clike",
	"java": "clike", "kotlin": "clike", "kt": "clike", "swift": "clike", "cs": "clike", "csharp": "clike", "php": "clike", "scala": "clike", "dart": "clike",
	"yml": "yaml", "toml": "yaml", "ini": "yaml", "bash": "sh", "shell": "sh", "zsh": "sh", "console": "sh", "fish": "sh", "sh": "sh", "psql": "sql", "mysql": "sql",
	"sql": "sql", "go ": "go", "json": "json", "yaml": "yaml", "rust": "rust", "python": "python", "go.mod": "go",
}

// langOf resolves the name after the opening fence; nil means no highlighting.
func langOf(name string) *codeLang {
	name = strings.ToLower(strings.TrimSpace(name))
	if l, ok := codeLangs[name]; ok {
		return l
	}
	return codeLangs[langAliases[name]]
}

var yamlKey = regexp.MustCompile("^(\\s*(?:-\\s+)?)([\\w.\\-/]+|\"[^\"]*\"|'[^']*')(:)(\\s|$)")

// highlighter colours lines of one block in order; strings and comments may
// span lines.
type highlighter struct {
	lang    *codeLang
	inBlock bool
	rawEnd  string
}

type spanKind int

const (
	spPlain spanKind = iota
	spKeyword
	spString
	spNumber
	spComment
	spFunc
	spLiteral
	spKey
	spPrompt
	spVar
)

func (k spanKind) seg(text string, bg tcell.Color) Seg {
	s := Seg{Text: text, FG: FgWhite, BG: bg}
	switch k {
	case spKeyword:
		s.FG = Purple
	case spString:
		s.FG = Green
	case spNumber, spLiteral, spVar:
		s.FG = Orange
	case spComment:
		s.FG, s.Italic = FgDim, true
	case spFunc, spKey:
		s.FG = Blue
	case spPrompt:
		s.FG, s.Bold = Green, true
	}
	return s
}

type spanList struct {
	segs []Seg
	bg   tcell.Color
}

func (l *spanList) add(k spanKind, text string) {
	if text == "" {
		return
	}
	s := k.seg(text, l.bg)
	if n := len(l.segs); n > 0 {
		last := l.segs[n-1]
		last.Text = ""
		want := s
		want.Text = ""
		if last == want {
			l.segs[n-1].Text += text
			return
		}
	}
	l.segs = append(l.segs, s)
}

func isIdentStart(r rune) bool { return r == '_' || r == '$' || unicode.IsLetter(r) }

func isIdent(r rune) bool { return isIdentStart(r) || unicode.IsDigit(r) }
