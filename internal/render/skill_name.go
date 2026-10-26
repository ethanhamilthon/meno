package render

import (
	"html"
	"strings"
)

// skillName is the name in the opening tag of a skill turn: <skill name="x" ...>.
func skillName(text string) string {
	_, rest, ok := strings.Cut(text, `name="`)
	if !ok {
		return "?"
	}
	name, _, _ := strings.Cut(rest, `"`)
	return html.UnescapeString(name)
}
