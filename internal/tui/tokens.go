package tui

import (
	"strings"
	"unicode"
)

// Skill (/skill:name) references can appear anywhere at a word boundary.
const skillPrefix = "/skill:"
const (
	kindSkill = '/'
)

type tokSpan struct {
	start, end int // rune offsets, sigil included
	kind       rune
	name       string
}

func isNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_'
}

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
		if in[i] != kindSkill || !wordStart(in, i) || !strings.HasPrefix(string(in[i:]), skillPrefix) {
			continue
		}
		e := i + len(skillPrefix)
		for e < len(in) && isNameRune(in[e]) {
			e++
		}
		// A suffix path is not a skill reference.
		if e < len(in) && in[e] == '/' {
			i = e
			continue
		}
		name := string(in[i+len(skillPrefix) : e])
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
	start := s - len(skillPrefix)
	if start < 0 || string(in[start:s]) != skillPrefix || !wordStart(in, start) {
		return acToken{}, false
	}
	e := c
	for e < len(in) && isNameRune(in[e]) {
		e++
	}
	if e < len(in) && in[e] == '/' {
		return acToken{}, false
	}
	return acToken{start: start, end: e, kind: kindSkill, prefix: strings.ToLower(string(in[s:c]))}, true
}

type acItem struct {
	kind       rune
	name, desc string
}

func (it acItem) label() string {
	if it.kind == kindSkill {
		return skillPrefix + it.name
	}
	return string(it.kind) + it.name
}

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
	}
	return out
}
