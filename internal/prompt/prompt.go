// Package prompt builds the system prompt sent to the model. Everything is
// assembled here, in code: Build is the single entry point and lists the
// sections in the order the model sees them, most stable first.
package prompt

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// identityPrompt is the first section: who the agent is, nothing else.
const identityPrompt = "You are meno, a coding agent."

// skillsPrompt lists the enabled skills (one line each).
func skillsPrompt(lines []string) string {
	return "## Skills\nA skill is an instruction file for one type of work. When the task matches a skill, read its file with fs.read before you start. Obey the file. If the user writes /name, read that skill first.\n" + strings.Join(lines, "\n")
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
	Cwd string
	// Subagent builds the prompt of a child agent: no askUser, tasks or
	// agents.spawn, and the child's own instructions.
	Subagent      bool
	Skills        []Skill
	GlobalAgents  Instructions
	ProjectAgents Instructions
	Tools         []Tool
	Now           time.Time
	OS, Shell     string
	// Model is the model this session talks to; empty omits the line.
	Provider, Model, Effort string
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
	add(guidelines(in.Subagent))
	add(jsTool(in.Subagent))
	if in.Subagent {
		add(subagentPrompt)
	} else {
		add(workPrompt)
	}
	add(mcpPrompt)
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

// mcpToolsSection lists the running servers with the names of their tools, up
// to maxListedTools names in all; mcpTools() gives the rest and the schemas.
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
	var servers []string
	byServer := map[string][]string{}
	for _, t := range tools {
		if _, ok := byServer[t.Server]; !ok {
			servers = append(servers, t.Server)
		}
		byServer[t.Server] = append(byServer[t.Server], ident(t.Name))
	}
	listed := 0
	var l []string
	for _, s := range servers {
		names := byServer[s]
		noun := "tools"
		if len(names) == 1 {
			noun = "tool"
		}
		line := fmt.Sprintf("- %s (%d %s)", ident(s), len(names), noun)
		if listed >= maxListedTools {
			l = append(l, line)
			continue
		}
		room := min(len(names), maxListedTools-listed)
		listed += room
		line += ": " + strings.Join(names[:room], ", ")
		if room < len(names) {
			line += fmt.Sprintf(", … and %d more", len(names)-room)
		}
		l = append(l, line)
	}
	return "## MCP servers\n" + strings.Join(l, "\n")
}

// ToolSignature renders one MCP tool as a call with its parameters.
func ToolSignature(t Tool) string {
	return "mcp." + ident(t.Server) + "." + ident(t.Name) + "(" + t.Params + ")"
}

func environmentSection(in Input) string {
	lines := []string{"## Environment", "- Working directory: " + in.Cwd}
	switch {
	case in.OS != "" && in.Shell != "":
		lines = append(lines, "- OS: "+in.OS+". Shell: "+in.Shell+".")
	case in.OS != "":
		lines = append(lines, "- OS: "+in.OS+".")
	case in.Shell != "":
		lines = append(lines, "- Shell: "+in.Shell+".")
	}
	if !in.Now.IsZero() {
		lines = append(lines, "- Date: "+FormatDate(in.Now)+".")
	}
	if in.Model != "" {
		lines = append(lines, "- Model: "+DescribeModel(in.Provider, in.Model, in.Effort)+".")
	}
	return strings.Join(lines, "\n")
}

// DescribeModel renders a model selection as "gpt-5.5 (openai, effort high)".
func DescribeModel(provider, model, effort string) string {
	var extra []string
	if provider != "" {
		extra = append(extra, provider)
	}
	if effort != "" {
		extra = append(extra, "effort "+effort)
	}
	if len(extra) == 0 {
		return model
	}
	return model + " (" + strings.Join(extra, ", ") + ")"
}

// FormatDate renders a date as "2026-10-17, Saturday (CEST)". No time of day:
// it would change the prompt on every build.
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02, Monday (MST)")
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

// guidelines is the Guidelines section. A subagent does not get the ask rule:
// it cannot ask the user.
func guidelines(subagent bool) string {
	l := []string{"## Guidelines"}
	l = append(l, baseGuidelines)
	if !subagent {
		l = append(l, askGuideline)
	}
	return strings.Join(l, "\n")
}

func jsTool(subagent bool) string {
	globals := jsGlobalsMain
	if subagent {
		globals = jsGlobalsChild
	}
	return jsToolPrompt + globals + jsHelpLine
}
