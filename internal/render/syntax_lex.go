package render

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// line colours one line; bg is the background of every segment.
func (h *highlighter) line(src string, bg tcell.Color) []Seg {
	out := &spanList{bg: bg}
	l := h.lang
	if l == nil {
		out.add(spPlain, src)
		return out.segs
	}
	rs := []rune(src)
	i := 0
	if l.shell {
		if t := strings.TrimLeft(src, " \t"); strings.HasPrefix(t, "$ ") {
			lead := len(src) - len(t)
			out.add(spPlain, src[:lead])
			out.add(spPrompt, "$")
			i = len([]rune(src[:lead])) + 1
		}
	}
	if l.yamlKeys && !h.inBlock && h.rawEnd == "" {
		if m := yamlKey.FindStringSubmatch(src); m != nil {
			out.add(spPlain, m[1])
			out.add(spKey, m[2])
			out.add(spPlain, m[3])
			i = len([]rune(m[1] + m[2] + m[3]))
		}
	}
	has := func(at int, s string) bool {
		if s == "" || at+len(s) > len(rs) {
			return false
		}
		for k, r := range []rune(s) {
			if l.fold && unicode.ToLower(rs[at+k]) != unicode.ToLower(r) || !l.fold && rs[at+k] != r {
				return false
			}
		}
		return true
	}
	for i < len(rs) {
		if h.inBlock {
			j := i
			for j < len(rs) && !has(j, l.blockEnd) {
				j++
			}
			if j < len(rs) {
				j += len([]rune(l.blockEnd))
				h.inBlock = false
			}
			out.add(spComment, string(rs[i:j]))
			i = j
			continue
		}
		if h.rawEnd != "" {
			j := i
			for j < len(rs) && !has(j, h.rawEnd) {
				if rs[j] == '\\' && h.rawEnd != "`" {
					j++
				}
				j++
			}
			j = min(j, len(rs))
			if j < len(rs) {
				j += len([]rune(h.rawEnd))
				h.rawEnd = ""
			}
			out.add(spString, string(rs[i:j]))
			i = j
			continue
		}
		c := rs[i]
		if c == ' ' || c == '\t' {
			j := i
			for j < len(rs) && (rs[j] == ' ' || rs[j] == '\t') {
				j++
			}
			out.add(spPlain, string(rs[i:j]))
			i = j
			continue
		}
		wordStart := i == 0 || rs[i-1] == ' ' || rs[i-1] == '\t'
		comment := false
		for _, lc := range l.lineComment {
			if has(i, lc) && (!l.hashNeedsSpace || wordStart) {
				comment = true
			}
		}
		if comment {
			out.add(spComment, string(rs[i:]))
			return out.segs
		}
		if l.blockStart != "" && has(i, l.blockStart) {
			h.inBlock = true
			out.add(spComment, l.blockStart)
			i += len([]rune(l.blockStart))
			continue
		}
		raw := ""
		for _, d := range l.raw {
			if has(i, d) {
				raw = d
				break
			}
		}
		if raw != "" {
			h.rawEnd = raw
			out.add(spString, raw)
			i += len([]rune(raw))
			continue
		}
		if strings.ContainsRune(l.quotes, c) {
			j := i + 1
			for j < len(rs) && rs[j] != c {
				if rs[j] == '\\' {
					j++
				}
				j++
			}
			j = min(j+1, len(rs))
			kind := spString
			if l.jsonKeys {
				for k := j; k < len(rs); k++ {
					if rs[k] == ' ' {
						continue
					}
					if rs[k] == ':' {
						kind = spKey
					}
					break
				}
			}
			out.add(kind, string(rs[i:j]))
			i = j
			continue
		}
		if l.shell && c == '$' && i+1 < len(rs) && (isIdentStart(rs[i+1]) || rs[i+1] == '{' || rs[i+1] == '(' || unicode.IsDigit(rs[i+1])) {
			j := i + 1
			if rs[j] == '{' || rs[j] == '(' {
				for j < len(rs) && rs[j] != '}' && rs[j] != ')' {
					j++
				}
				j = min(j+1, len(rs))
			} else {
				for j < len(rs) && isIdent(rs[j]) {
					j++
				}
			}
			out.add(spVar, string(rs[i:j]))
			i = j
			continue
		}
		if unicode.IsDigit(c) && (i == 0 || !isIdent(rs[i-1])) {
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '.' || rs[j] == '_') {
				j++
			}
			out.add(spNumber, string(rs[i:j]))
			i = j
			continue
		}
		if isIdentStart(c) {
			j := i
			for j < len(rs) && (isIdent(rs[j]) || l.shell && rs[j] == '-') {
				j++
			}
			word := string(rs[i:j])
			key := word
			if l.fold {
				key = strings.ToLower(word)
			}
			next := j
			for next < len(rs) && rs[next] == ' ' {
				next++
			}
			switch {
			case l.keywords[key]:
				out.add(spKeyword, word)
			case l.literals[key]:
				out.add(spLiteral, word)
			case next < len(rs) && rs[next] == '(' && !l.shell:
				out.add(spFunc, word)
			default:
				out.add(spPlain, word)
			}
			i = j
			continue
		}
		out.add(spPlain, string(c))
		i++
	}
	return out.segs
}
