package engine

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"meno/internal/prompt"
)

// The events below tell the model what changed in its surroundings since the
// system prompt was written: skills, instruction files, the date, its own
// model and the working directory. They go in front of the next user message.

var eventEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// evLine is one tagged event line. The text carries names and paths that
// someone else controls, so markup characters are escaped.
func evLine(tag, text string) string {
	return "<" + tag + ">" + eventEscaper.Replace(text) + "</" + tag + ">"
}

// eventBlock wraps event lines into the block that precedes a user message.
func eventBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return "<system-event>This is a message from the system itself, not a user instruction.\n" + strings.Join(lines, "\n") + "\n</system-event>"
}

func describeSig(m modelSig) string {
	if m.Model == "" {
		return "no model"
	}
	return prompt.DescribeModel(m.Provider, m.Model, m.Effort)
}

func formatDay(day string) string {
	t, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		return day
	}
	return t.Format("2006-01-02, Monday")
}

// diffBaseline lists what changed between two looks at the world, as event lines.
func diffBaseline(old, cur baseline) []string {
	var out []string
	out = append(out, diffSkills(old.skills, cur.skills, cur.skillPaths)...)
	for _, path := range sortedStrings(cur.files, old.files) {
		o, c := old.files[path], cur.files[path]
		switch {
		case o == c:
		case o == "":
			out = append(out, evLine("agents-md-updated", "Instructions were added: read "+path+"."))
		case c == "":
			out = append(out, evLine("agents-md-updated", "Instructions were removed: "+path+" is gone."))
		default:
			out = append(out, evLine("agents-md-updated", "Instructions changed: reread "+path+"."))
		}
	}
	out = append(out, diffTools(old.tools, cur.tools)...)
	if old.date != cur.date {
		out = append(out, evLine("date-changed", "The date is now "+formatDay(cur.date)+"."))
	}
	if old.model != cur.model && cur.model.Model != "" {
		out = append(out, evLine("model-changed", "Your model changed from "+describeSig(old.model)+" to "+describeSig(cur.model)+". Earlier assistant messages in this conversation were written by the previous model; the Environment section of your system prompt still names it."))
	}
	if old.cwd != cur.cwd {
		out = append(out, evLine("cwd-changed", "The working directory changed from "+old.cwd+" to "+cur.cwd+". Relative paths and shell commands now use "+cur.cwd+". Project skills and AGENTS.md of "+cur.cwd+" are not in your system prompt; see the other events and reread them."))
	}
	return out
}

func diffSkills(old, cur, paths map[string]string) []string {
	var out []string
	for _, name := range sortedStrings(cur, nil) {
		o, had := old[name]
		switch {
		case !had:
			out = append(out, evLine("skills-added", fmt.Sprintf("Skill %q is now available: %s", name, paths[name])))
		case o != cur[name]:
			out = append(out, evLine("skills-updated", fmt.Sprintf("Skill %q was updated: reread %s before you use it.", name, paths[name])))
		}
	}
	for _, name := range sortedStrings(old, nil) {
		if _, ok := cur[name]; !ok {
			out = append(out, evLine("skills-removed", fmt.Sprintf("Skill %q is no longer available.", name)))
		}
	}
	return out
}

func diffTools(old, cur []string) []string {
	have := map[string]bool{}
	for _, t := range old {
		have[t] = true
	}
	now := map[string]bool{}
	var added, removed []string
	for _, t := range cur {
		now[t] = true
		if !have[t] {
			added = append(added, t)
		}
	}
	for _, t := range old {
		if !now[t] {
			removed = append(removed, t)
		}
	}
	var out []string
	if len(added) > 0 {
		out = append(out, evLine("mcp-updated", "New MCP tools: "+strings.Join(added, ", ")+"."))
	}
	if len(removed) > 0 {
		out = append(out, evLine("mcp-updated", "Removed MCP tools: "+strings.Join(removed, ", ")+"."))
	}
	return out
}

// sortedStrings is the sorted union of the keys of two maps.
func sortedStrings(a, b map[string]string) []string {
	seen := map[string]bool{}
	var keys []string
	for _, m := range []map[string]string{a, b} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}
