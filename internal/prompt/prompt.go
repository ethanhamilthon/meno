// Package prompt holds the texts sent to the model. The texts live in
// the .md files next to this file and are embedded at build time.
package prompt

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed system.md
var system string

//go:embed skills.md
var skillsSection string

//go:embed mcp.md
var mcpSection string

//go:embed mcp_admin.md
var mcpAdminSection string

// Skill is one enabled skill as the model sees it.
type Skill struct {
	Name, Desc, Path string
}

// Tool is one MCP tool as the model sees it. Params is a short signature such
// as "{query: string, limit?: number}".
type Tool struct {
	Server, Name, Desc, Params string
}

// maxListedTools bounds how many tools are spelled out in the prompt; the rest
// can be found with mcpTools().
const maxListedTools = 60

// System returns the base system prompt for the given working directory.
func System(cwd string) string {
	return strings.ReplaceAll(strings.TrimSpace(system), "{{cwd}}", cwd)
}

// Build returns the full system prompt: the base prompt followed by the
// sections for enabled skills and running MCP servers, when there are any.
func Build(cwd string, skills []Skill, tools []Tool) string {
	parts := []string{System(cwd), strings.TrimSpace(mcpAdminSection)}
	if len(skills) > 0 {
		var l []string
		for _, s := range skills {
			line := "- " + s.Name
			if d := short(s.Desc, 200); d != "" {
				line += " — " + d
			}
			l = append(l, line+" ("+s.Path+")")
		}
		parts = append(parts, fill(skillsSection, l))
	}
	if len(tools) > 0 {
		var l []string
		for i, t := range tools {
			if i == maxListedTools {
				l = append(l, fmt.Sprintf("- … and %d more: see mcpTools()", len(tools)-i))
				break
			}
			line := "- mcp." + ident(t.Server) + "." + ident(t.Name) + "(" + t.Params + ")"
			if d := short(t.Desc, 160); d != "" {
				line += " — " + d
			}
			l = append(l, line)
		}
		parts = append(parts, fill(mcpSection, l))
	}
	return strings.Join(parts, "\n\n")
}

func fill(tmpl string, lines []string) string {
	return strings.ReplaceAll(strings.TrimSpace(tmpl), "{{list}}", strings.Join(lines, "\n"))
}

// short flattens s to one line of at most n runes.
func short(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// Ident returns the name under which the js side exposes a server or tool:
// names that are valid identifiers stay, anything else gets underscores.
func Ident(name string) string { return ident(name) }

func ident(name string) string {
	var b strings.Builder
	for i, r := range name {
		ok := r == '_' || r == '$' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9'
		if ok {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}
