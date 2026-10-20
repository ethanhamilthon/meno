// Package prompt builds the system prompt sent to the model. Everything is
// assembled here, in code: Build is the single entry point and lists the
// sections in the order the model sees them, most stable first.
package prompt

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// identityPrompt is the first section: who the agent is, nothing else.
const identityPrompt = "You are meno, a coding agent."

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
	// Only lists the tool globals this prompt keeps; nil keeps them all.
	Only []string
	// Tool selects the bash prompt when set to bash; empty preserves js.
	Tool string
	// Allow lists bash command groups; nil allows every group.
	Allow []string
	// Model is the model this session talks to; empty omits the line.
	Provider, Model, Effort string
}

// Build returns the full system prompt. Sections go from the most stable to
// the most volatile so that a change only invalidates the text after it.
func Build(in Input) string { return buildBash(in) }

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
