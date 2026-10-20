package llm

import (
	"encoding/json"
	"strings"
	"unicode"
)

// BashToolName is the name of the shell tool.
const BashToolName = "bash"

// toolSchema is the definition of the tool a request offers.
func (r Request) toolSchema() map[string]any {
	if r.ToolName == BashToolName {
		return BashSchema(r.ToolDescription)
	}
	return ToolSchema(r.ToolDescription)
}

// BashSchema is the definition of the bash tool: a command. An empty desc gives a short default.
func BashSchema(desc string) map[string]any {
	if desc == "" {
		desc = "Run a shell command in the session working directory."
	}
	props := map[string]any{
		"title":   map[string]any{"type": "string", "description": "Short English action label, without secrets or command arguments."},
		"command": map[string]any{"type": "string", "description": "Shell command to run."},
	}
	required := []string{"command", "title"}
	return map[string]any{
		"type":        "function",
		"name":        BashToolName,
		"description": desc,
		"parameters": map[string]any{
			"type":       "object",
			"properties": props,
			"required":   required,
		},
	}
}

// BashTitle never exposes command arguments in a fallback label.
func BashTitle(args string) string {
	var supplied struct {
		Title string `json:"title"`
	}
	if json.Unmarshal([]byte(args), &supplied) == nil {
		title := strings.Join(strings.Fields(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) && !unicode.IsSpace(r) {
				return -1
			}
			return r
		}, supplied.Title)), " ")
		if title != "" {
			r := []rune(title)
			if len(r) > 80 {
				r = r[:80]
			}
			return string(r)
		}
	}
	fields := strings.Fields(ToolArgs(args))
	if len(fields) == 0 {
		return "Bash command"
	}
	word := fields[0]
	for _, r := range word {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_-.", r)) {
			return "Bash command"
		}
	}
	if len(word) > 40 {
		return "Bash command"
	}
	return "Run " + word
}
