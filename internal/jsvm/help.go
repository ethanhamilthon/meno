package jsvm

import (
	"embed"
	"fmt"
	"strings"

	"github.com/dop251/goja"
)

//go:embed help/*.md
var helpFS embed.FS

type helpTopic struct {
	name, line string
	// file is the topic text in help/; a subagent can use childFile instead.
	file, childFile string
	// mainOnly topics are not offered to a subagent.
	mainOnly bool
}

// helpTopics are the topics of help(), in the order help() lists them.
var helpTopics = []helpTopic{
	{name: "js", line: "runtime rules, sh, view, fetch, env", file: "js.md"},
	{name: "fs", line: "read, inspect, search, patch, edit, write, glob, exists", file: "fs.md"},
	{name: "session", line: "models", file: "session.md", childFile: "session-child.md"},
	{name: "askUser", line: "ask the user questions in one form", file: "askUser.md", mainOnly: true},
	{name: "tasks", line: "background shell commands", file: "tasks.md", mainOnly: true},
	{name: "agents", line: "subagents", file: "agents.md", childFile: "agents-child.md"},
	{name: "mcp", line: "call MCP tools, read cut results, schemas", file: "mcp.md"},
	{name: "mcpServers", line: "list, add, update, remove, restart MCP servers", file: "mcpServers.md"},
}

// HelpTopics returns the names help() lists, for the prompt and tests.
func HelpTopics(child bool) []string {
	var out []string
	for _, t := range helpTopics {
		if child && t.mainOnly {
			continue
		}
		out = append(out, t.name)
	}
	return out
}

func helpText(name string, child bool) (string, error) {
	if name == "" {
		var b strings.Builder
		b.WriteString("Topics (call help(\"name\")):\n")
		for _, t := range helpTopics {
			if child && t.mainOnly {
				continue
			}
			fmt.Fprintf(&b, "- %s: %s\n", t.name, t.line)
		}
		return b.String(), nil
	}
	for _, t := range helpTopics {
		if t.name != name || (child && t.mainOnly) {
			continue
		}
		file := t.file
		if child && t.childFile != "" {
			file = t.childFile
		}
		b, err := helpFS.ReadFile("help/" + file)
		return string(b), err
	}
	return "", fmt.Errorf("help: unknown topic %q (topics: %s)", name, strings.Join(HelpTopics(child), ", "))
}

func (v *VM) bindHelp() {
	rt := v.rt
	_ = rt.Set("help", func(c goja.FunctionCall) goja.Value {
		name := ""
		if len(c.Arguments) > 0 && !goja.IsUndefined(c.Arguments[0]) {
			s, ok := c.Arguments[0].Export().(string)
			if !ok {
				panic(rt.NewTypeError("help: the topic must be a string"))
			}
			name = s
		}
		text, err := helpText(name, v.agentsChild.Load())
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(text)
	})
}
