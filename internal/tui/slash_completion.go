package tui

import (
	"meno/internal/content"
	"sort"
	"strings"
)

// skillRows are the enabled skills whose name continues the command query, as
// "/skill:name" rows for the command popup: typing "/pla" finds the skill
// "plan" without typing the "skill:" prefix. A command named exactly like the
// query wins, and nothing is offered for an empty query or a command with
// arguments.
func (a *App) skillRows(query string) []string {
	q := strings.ToLower(query)
	if q == "" || strings.ContainsAny(q, " \t:/") {
		return nil
	}
	for _, c := range commandList {
		if c.name == "/"+q {
			return nil
		}
	}
	var out []string
	for _, s := range a.st.Skills {
		if s.Enabled && strings.HasPrefix(strings.ToLower(s.Name), q) {
			out = append(out, skillPrefix+s.Name)
		}
	}
	sort.Strings(out)
	return out
}

// skillRowName is the skill a command row stands for, and whether it is one.
// The "/skill:" row itself is the command that starts a reference, not a skill.
func skillRowName(row string) (string, bool) {
	name, ok := strings.CutPrefix(row, skillPrefix)
	return name, ok && name != ""
}

// skillDesc is the description of an enabled skill.
func (a *App) skillDesc(name string) string {
	for _, s := range a.st.Skills {
		if s.Name == name {
			return s.Desc
		}
	}
	return ""
}

// cmdInsertSkill replaces the command token with the skill as one atomic
// object. It never runs or sends anything.
func (a *App) cmdInsertSkill(name string) {
	t, ok := a.cmdToken()
	if !ok {
		return
	}
	rep := []rune(skillPrefix + name)
	if r, ok := objectRune(inputObject{Kind: content.Skill, Ref: name}); ok {
		rep = []rune{r}
	}
	in := a.st.Input
	tail := in[t.end:]
	out := append(append([]rune(nil), in[:t.start]...), rep...)
	cursor := len(out)
	if len(tail) == 0 || tail[0] != ' ' {
		out = append(out, ' ')
	}
	cursor++
	out = append(out, tail...)
	a.endInsert()
	a.st.Input, a.st.Cursor, a.st.WantSet = out, cursor, false
	a.st.InsBase = a.snapshot()
	a.st.CmdSel, a.st.CmdPicked = 0, false
}

// cmdPick is Tab on a command row: a skill becomes an object, a command is
// completed in the text.
func (a *App) cmdPick(row string) {
	if name, ok := skillRowName(row); ok {
		a.cmdInsertSkill(name)
		return
	}
	a.cmdComplete(row)
}
