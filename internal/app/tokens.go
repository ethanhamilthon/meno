package app

import (
	"strings"
	"unicode"

	"meno/internal/modes"
)

// Skill (/name) references can appear anywhere in the input,
// as long as the sigil starts a word. A mode (#name) is only the first word.
const (
	kindSkill = '/'
	kindMode  = '#'
)

type tokSpan struct {
	start, end int // rune offsets, sigil included
	kind       rune
	name       string
}

func isNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_'
}

func isSigil(r rune) bool { return r == kindSkill || r == kindMode }

func wordStart(in []rune, i int) bool { return i == 0 || unicode.IsSpace(in[i-1]) }

func (a *App) skillEnabled(name string) bool {
	for _, s := range a.st.Skills {
		if s.Enabled && s.Name == name {
			return true
		}
	}
	return false
}

// inputTokens returns the complete, existing skill references.
func (a *App) inputTokens() []tokSpan {
	in := a.st.Input
	var out []tokSpan
	for i := 0; i < len(in); i++ {
		if !isSigil(in[i]) || !wordStart(in, i) {
			continue
		}
		e := i + 1
		for e < len(in) && isNameRune(in[e]) {
			e++
		}
		// "/api-security/foo" is a path, not a skill reference.
		if e < len(in) && in[e] == '/' {
			i = e
			continue
		}
		name := string(in[i+1 : e])
		ok := name != "" && (in[i] == kindSkill && a.skillEnabled(name))
		if ok {
			out = append(out, tokSpan{i, e, in[i], name})
		}
		i = e - 1
	}
	return append(out, a.imageSpans()...)
}

// acToken is the partial reference around the cursor: [start,end) covers the
// whole word, prefix is the text between the sigil and the cursor.
type acToken struct {
	start, end int
	kind       rune
	prefix     string
}

func (a *App) acToken() (acToken, bool) {
	if a.pickOpen() {
		if s, e, q, ok := a.pickToken(); ok {
			return acToken{start: s, end: e, kind: '@', prefix: q}, true
		}
	}
	in := a.st.Input
	c := min(max(0, a.st.Cursor), len(in))
	s := c
	for s > 0 && isNameRune(in[s-1]) {
		s--
	}
	if s == 0 || !isSigil(in[s-1]) || !wordStart(in, s-1) {
		return acToken{}, false
	}
	e := c
	for e < len(in) && isNameRune(in[e]) {
		e++
	}
	return acToken{start: s - 1, end: e, kind: in[s-1], prefix: strings.ToLower(string(in[s:c]))}, true
}

type acItem struct {
	kind       rune
	name, desc string
}

func (it acItem) label() string { return string(it.kind) + it.name }

func (a *App) acMatches() []acItem {
	if a.pickOpen() {
		return a.pickItems()
	}
	t, ok := a.acToken()
	if !ok {
		return nil
	}
	var out []acItem
	switch t.kind {
	case kindSkill:
		for _, s := range a.st.Skills {
			if s.Enabled && strings.HasPrefix(strings.ToLower(s.Name), t.prefix) {
				out = append(out, acItem{kindSkill, s.Name, s.Desc})
			}
		}
	case kindMode:
		for _, m := range a.modeCatalog() {
			if strings.HasPrefix(m.Name, t.prefix) {
				out = append(out, acItem{kindMode, m.Name, m.Description})
			}
		}
		if strings.HasPrefix(modes.Control, t.prefix) {
			out = append(out, acItem{kindMode, modes.Control, "clear the modes of this session"})
		}
	}
	return out
}
