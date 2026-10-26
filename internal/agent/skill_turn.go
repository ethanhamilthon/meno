package agent

import (
	"html"
	"path/filepath"
	"strings"

	"meno/internal/llm"
)

// KindSkill marks a user-role turn that carries the text of a skill the user
// named with /skill:name. The screen shows only its name.
const KindSkill = "skill"

// SkillTurn loads a skill into the conversation: the text of its file, with the
// path the model needs to find the files next to it.
func SkillTurn(name, path, body string) Turn {
	var b strings.Builder
	b.WriteString(`<skill name="` + html.EscapeString(name) + `" path="` + html.EscapeString(path) + `">` + "\n")
	b.WriteString("Relative paths in this skill start at " + filepath.Dir(path) + ".\n\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n</skill>")
	return Turn{Role: llm.RoleUser, Kind: KindSkill, Text: b.String(), Time: nowUTC()}
}
