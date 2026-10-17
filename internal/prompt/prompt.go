// Package prompt builds the system prompt sent to the model. Everything is
// assembled here, in code: Build is the single entry point and lists the
// sections in the order the model sees them, most stable first.
package prompt

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

// identityPrompt is the first section: who the agent is, nothing else.
const identityPrompt = "You are meno, a coding agent."

// jsToolPrompt documents the only tool and the globals of its VM.
const jsToolPrompt = `## The js tool
Your only tool is js. It runs JavaScript in a persistent VM: top-level let, const and function declarations survive between calls. Everything is synchronous: there is no await and there are no Promises. The value of the last expression is returned. Output is capped at 64 KB per call and every call has a timeout.
Globals:
- fs.read(path), fs.write(path, text), fs.edit(path, old, new), fs.glob(pattern), fs.exists(path): files at any path; relative paths resolve against the working directory. fs.edit needs a unique match.
- sh(cmd) -> {stdout, stderr, code, timedOut}: run a shell command in the working directory.
- fetch(url, init), env, URL, URLSearchParams, console.log(...).
- askUser, mcpServers and mcp: described below.`

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

// Instructions is the content of one AGENTS.md. The zero value means "none".
type Instructions struct {
	Path, Text string
}

// maxListedTools bounds how many tools are spelled out in the prompt; the rest
// can be found with mcpTools().
const maxListedTools = 60

// MaxInstructions bounds how much of one AGENTS.md is used.
const MaxInstructions = 256 << 10

// Input is everything the system prompt depends on.
type Input struct {
	Cwd           string
	Skills        []Skill
	GlobalAgents  Instructions
	ProjectAgents Instructions
	Tools         []Tool
	Now           time.Time
	OS, Shell     string
}

// Build returns the full system prompt. Sections go from the most stable to
// the most volatile so that a change only invalidates the text after it.
func Build(in Input) string {
	var parts []string
	add := func(s string) {
		if s != "" {
			parts = append(parts, s)
		}
	}
	add(identityPrompt)
	add(jsToolPrompt)
	add(askPrompt)
	add(mcpAdminPrompt)
	add(skillsSection(in.Skills))
	add(instructionsSection("Global instructions", in.GlobalAgents))
	add(instructionsSection("Project instructions", in.ProjectAgents))
	add(mcpToolsSection(in.Tools))
	add(environmentSection(in))
	return strings.Join(parts, "\n\n")
}

func skillsSection(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	skills = append([]Skill(nil), skills...)
	sort.SliceStable(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	var l []string
	for _, s := range skills {
		line := "- " + s.Name
		if d := short(s.Desc, 200); d != "" {
			line += " — " + d
		}
		l = append(l, line+" ("+s.Path+")")
	}
	return skillsPrompt(l)
}

func instructionsSection(title string, in Instructions) string {
	if strings.TrimSpace(in.Text) == "" {
		return ""
	}
	return "## " + title + "\nFrom " + in.Path + ". Follow these unless the user's message says otherwise.\n\n" + strings.TrimRight(in.Text, "\n")
}

func mcpToolsSection(tools []Tool) string {
	if len(tools) == 0 {
		return ""
	}
	tools = append([]Tool(nil), tools...)
	sort.SliceStable(tools, func(i, j int) bool {
		if tools[i].Server != tools[j].Server {
			return tools[i].Server < tools[j].Server
		}
		return tools[i].Name < tools[j].Name
	})
	var l []string
	for i, t := range tools {
		if i == maxListedTools {
			l = append(l, fmt.Sprintf("- … and %d more: see mcpTools()", len(tools)-i))
			break
		}
		l = append(l, ToolLine(t))
	}
	return mcpToolsPrompt(l)
}

// ToolLine renders one MCP tool the way the prompt (and system events) list it.
func ToolLine(t Tool) string {
	line := "- mcp." + ident(t.Server) + "." + ident(t.Name) + "(" + t.Params + ")"
	if d := short(t.Desc, 160); d != "" {
		line += " — " + d
	}
	return line
}

// ToolSignature is ToolLine without the bullet and the description.
func ToolSignature(t Tool) string {
	return "mcp." + ident(t.Server) + "." + ident(t.Name) + "(" + t.Params + ")"
}

func environmentSection(in Input) string {
	var b strings.Builder
	b.WriteString("## Environment\nWorking directory: " + in.Cwd)
	if in.OS != "" {
		b.WriteString("\nOS: " + in.OS)
	}
	if in.Shell != "" {
		b.WriteString("\nShell: " + in.Shell)
	}
	if !in.Now.IsZero() {
		b.WriteString("\nDate: " + FormatDate(in.Now))
	}
	return b.String()
}

// FormatDate renders a date as "2026-10-17, Saturday (CEST)". No time of day:
// it would change the prompt on every build.
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02, Monday (MST)")
}

// DescribeOS returns e.g. "macOS 15.1 (darwin/arm64)"; when the version
// cannot be found, just "darwin/arm64".
func DescribeOS() string {
	plat := runtime.GOOS + "/" + runtime.GOARCH
	name := ""
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
			if v := strings.TrimSpace(string(out)); v != "" {
				name = "macOS " + v
			}
		}
	case "linux":
		if b, err := os.ReadFile("/etc/os-release"); err == nil {
			for _, l := range strings.Split(string(b), "\n") {
				if v, ok := strings.CutPrefix(l, "PRETTY_NAME="); ok {
					name = strings.Trim(strings.TrimSpace(v), "\"'")
				}
			}
		}
	}
	if name == "" {
		return plat
	}
	return name + " (" + plat + ")"
}

// LoadInstructions reads an AGENTS.md, cutting it at MaxInstructions. A missing
// or empty file gives the zero value and no error; an unreadable one gives an
// error.
func LoadInstructions(path string) (Instructions, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Instructions{}, nil
		}
		return Instructions{}, err
	}
	if strings.TrimSpace(string(b)) == "" {
		return Instructions{}, nil
	}
	text := string(b)
	if len(b) > MaxInstructions {
		cut := MaxInstructions
		for cut > 0 && !utf8Start(b[cut]) {
			cut--
		}
		text = string(b[:cut]) + fmt.Sprintf("\n[truncated: %d bytes more]", len(b)-cut)
	}
	return Instructions{Path: path, Text: text}, nil
}

// utf8Start reports whether b can begin a UTF-8 sequence.
func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

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
