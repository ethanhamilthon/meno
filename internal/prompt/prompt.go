// Package prompt builds the system prompt sent to the model. Everything is
// assembled here, in code: Build is the single entry point and lists the
// sections in the order the model sees them.
package prompt

import (
	"fmt"
	"strings"
)

// basePrompt introduces the agent and its only tool.
func basePrompt(cwd string) string {
	return "You are meno, a coding agent. Your only tool is js: run JavaScript with fs.read/write/edit/glob/exists, sh(cmd), fetch, env, URL and console.log. Working directory: " + cwd + "."
}

// askPrompt documents askUser.
const askPrompt = `## Asking the user
askUser(question, {choices?: string[], customAnswer?: boolean, timeout?: seconds}) pauses your code, shows the question to the user and returns the answer as a string: free text, or one of choices (with choices, the user may also type their own answer unless customAnswer is false). Use it only when the answer cannot be found by reading files, running commands or searching the project, and never to ask permission for ordinary actions. One uncertainty per question, short, with choices when there are few options. If the user declines (error "user declined to answer") or does not answer in time, do not repeat the question: continue with a sensible default or explain what you need.`

// mcpAdminPrompt documents mcpServers.
const mcpAdminPrompt = `## Managing MCP servers
mcpServers manages the MCP server configuration; changes are saved, applied at once and shown to the user. Every method is synchronous.
- mcpServers.list() — every configured server: {name, transport, command|url, enabled, status: "running"|"stopped"|"starting"|"failed", error?, tools: <count>, env?, headers?}. Secret values of env and headers are always shown as "***".
- mcpServers.add({name, transport?: "stdio"|"sse"|"streamable-http", command?, url?, env?: {K: "v"}, headers?: {K: "v"}, enabled?: true}) — add and start a server. command is a shell-style command line for stdio; url is for sse and streamable-http.
- mcpServers.update(name, patch) — change any of those fields (env and headers are replaced as a whole; enabled: false stops the server) and restart it.
- mcpServers.remove(name) and mcpServers.restart(name).
add, update and restart wait until the server is up and return {ok, error?, tools: [{name, description}]}; they do not throw. After success the tools are callable at once as mcp.<server>.<tool>() (re-read the mcp global rather than keeping a reference to it). mcpTools("name") lists the tools of one server with their schemas. Do not add, change or remove servers unless the user asked for it.`

// skillsPrompt lists the enabled skills (one line each).
func skillsPrompt(lines []string) string {
	return "## Skills\nSkills are instruction files for particular kinds of work. When the task matches one, read its file with fs.read before you start and follow it. When the user writes /name, that skill is requested explicitly: read it first.\n\n" + strings.Join(lines, "\n")
}

// mcpToolsPrompt lists the tools of the running MCP servers (one line each).
func mcpToolsPrompt(lines []string) string {
	return "## MCP tools\nCall them from js as mcp.<server>.<tool>({...}). Calls are synchronous and return the result text (JSON.parse it when it is JSON); a failed call throws. mcpTools() returns every tool with its full input schema.\n\n" + strings.Join(lines, "\n")
}

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
func System(cwd string) string { return basePrompt(cwd) }

// Build returns the full system prompt: the base prompt followed by the
// sections for enabled skills and running MCP servers, when there are any.
func Build(cwd string, skills []Skill, tools []Tool) string {
	parts := []string{System(cwd), askPrompt, mcpAdminPrompt}
	if len(skills) > 0 {
		var l []string
		for _, s := range skills {
			line := "- " + s.Name
			if d := short(s.Desc, 200); d != "" {
				line += " — " + d
			}
			l = append(l, line+" ("+s.Path+")")
		}
		parts = append(parts, skillsPrompt(l))
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
		parts = append(parts, mcpToolsPrompt(l))
	}
	return strings.Join(parts, "\n\n")
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
