package app

import (
	"meno/internal/ui"
	"strings"
	"unicode"
)

// cmdTok is the inline command around the cursor: a ':' that starts a word,
// followed by the command name. Commands that take arguments ("cd /tmp",
// "rename my title") span to the end of the line.
type cmdTok struct {
	start, end int
	query      string // text after ':'
	args       bool   // a known command name followed by a space
}

func isCommandName(word string) bool {
	for _, c := range commandList {
		if c.name == ":"+word {
			return true
		}
	}
	return false
}

func isBreak(r rune) bool { return unicode.IsSpace(r) || isObjRune(r) }

// cmdToken finds the command token the cursor is in or right after.
func (a *App) cmdToken() (cmdTok, bool) {
	in := a.st.Input
	c := min(max(0, a.st.Cursor), len(in))
	ls := c
	for ls > 0 && in[ls-1] != '\n' {
		ls--
	}
	for s := c - 1; s >= ls; s-- {
		if in[s] != ':' || (s > ls && !isBreak(in[s-1])) {
			continue
		}
		q := in[s+1 : c]
		word := string(q)
		if i := strings.IndexFunc(word, isBreak); i >= 0 {
			word = word[:i]
			if !isCommandName(word) || word == "" || !unicode.IsSpace([]rune(string(q))[len([]rune(word))]) {
				continue
			}
			e := c
			for e < len(in) && in[e] != '\n' {
				e++
			}
			return cmdTok{s, e, string(in[s+1 : e]), true}, true
		}
		e := c
		for e < len(in) && !isBreak(in[e]) {
			e++
		}
		return cmdTok{s, e, string(q), false}, true
	}
	return cmdTok{}, false
}

// cmdQuery is the text after the colon, "" when there is no token.
func (a *App) cmdQuery() string {
	t, _ := a.cmdToken()
	return t.query
}

// cmdActive reports whether the command popup owns Enter, Tab, Esc and the arrows.
func (a *App) cmdActive() bool {
	if a.st.Ask != nil || a.st.Over != OverNone || a.st.Mode != ui.ModeInput {
		return false
	}
	t, ok := a.cmdToken()
	if !ok || a.st.CmdDismiss == t.start+1 {
		return false
	}
	return t.args || len(commandRows(a)) > 0
}

// cmdSync forgets a dismissed popup once its token is gone.
func (a *App) cmdSync() {
	if _, ok := a.cmdToken(); !ok {
		a.st.CmdDismiss = 0
	}
}

// completionHeight is the popup above the composer: command rows, completions
// or the queue.
func (a *App) completionHeight(w int) int {
	if a.cmdActive() {
		return acRowCount(len(commandRows(a)), w)
	}
	return a.acHeight()
}
