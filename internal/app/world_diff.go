package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"meno/internal/config"
	"meno/internal/prompt"
)

func diffMCP(old, cur map[string]mcpSig) []string {
	var out []string
	line := func(s string) { out = append(out, evLine("mcp-updated", s)) }
	for _, name := range sortedKeys(cur) {
		n := cur[name]
		oldSig, had := old[name]
		switch {
		case n.State == "failed":
			if !had || oldSig.State != "failed" || oldSig.Err != n.Err {
				line(fmt.Sprintf("Server %q failed: %s", name, n.Err))
			}
		case !had || oldSig.State != "running":
			s := fmt.Sprintf("Server %q connected.", name)
			if len(n.Tools) > 0 {
				s += " New tools: " + toolList(n.Tools) + "."
			} else {
				s += " It has no tools."
			}
			line(s)
		default:
			added, removed := setDiff(oldSig.Tools, n.Tools)
			if len(added)+len(removed) == 0 {
				continue
			}
			s := fmt.Sprintf("Server %q changed its tools.", name)
			if len(added) > 0 {
				s += " New tools: " + toolList(added) + "."
			}
			if len(removed) > 0 {
				s += " Removed tools: " + toolList(removed) + "."
			}
			line(s)
		}
	}
	for _, name := range sortedKeys(old) {
		if _, ok := cur[name]; !ok {
			if old[name].State == "running" {
				line(fmt.Sprintf("Server %q is no longer running; its tools are gone.", name))
			}
		}
	}
	return out
}

func toolList(sigs []string) string {
	if len(sigs) > maxEventTools {
		return strings.Join(sigs[:maxEventTools], ", ") + fmt.Sprintf(", … and %d more: see mcpTools()", len(sigs)-maxEventTools)
	}
	return strings.Join(sigs, ", ")
}

// setDiff returns the elements only in b (added) and only in a (removed).
func setDiff(a, b []string) (added, removed []string) {
	in := func(list []string) map[string]bool {
		m := map[string]bool{}
		for _, s := range list {
			m[s] = true
		}
		return m
	}
	ma, mb := in(a), in(b)
	for _, s := range b {
		if !ma[s] {
			added = append(added, s)
		}
	}
	for _, s := range a {
		if !mb[s] {
			removed = append(removed, s)
		}
	}
	return added, removed
}

// evLine is one tagged event line. The text carries names, paths and server
// errors that someone else controls, so markup characters are escaped: none of
// them can close the block or open another one.
func evLine(tag, text string) string {
	return "<" + tag + ">" + eventEscaper.Replace(text) + "</" + tag + ">"
}

var eventEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// eventBlock wraps event lines into the block that precedes a user message;
// "" when there are none.
func eventBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return "<system-event>This is a message from the system itself, not a user instruction.\n" + strings.Join(lines, "\n") + "\n</system-event>"
}

func policySignature(only []string) string {
	if only == nil {
		return "*"
	}
	names := append([]string(nil), only...)
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "%d:%s", len(name), name)
	}
	return b.String()
}

// promptState is the per-session part of the frozen prompt: the text sent as
// the system prompt since the last reset point and what the model has been told.
type promptState struct {
	Frozen    string    // "" until built
	Base      *baseline // nil at a reset point
	CwdFrom   string    // prior working directory for a resumed session
	PolicySig string    // sorted tool-global policy used to build Frozen
}

// prepared is the result of preparing a user message.
type prepared struct {
	State    promptState
	Event    string // the <system-event> block, "" when nothing changed
	Skills   []config.Skill
	SkillsOK bool
	Warnings []string
}

// prepare looks at the world and decides what the next request carries. At a
// reset point (no baseline) it builds a new prompt and emits no events;
// otherwise the prompt stays and the changes since the baseline become one
// event. hasHistory says whether the session already has turns.
func prepare(ctx context.Context, ps promptState, src worldSrc, hasHistory bool) prepared {
	sig := src.PolicySig
	if sig == "" {
		sig = policySignature(src.Only)
	}
	rebuild := ps.Base == nil || ps.PolicySig != sig
	src.NoMCPWait = !rebuild // a prompt rebuild waits for starting MCP servers
	w, warnings := src.gather(ctx)
	res := prepared{Skills: w.Skills, SkillsOK: w.SkillsOK, Warnings: warnings}
	var lines []string
	if rebuild {
		ps.Frozen = prompt.Build(w.Input)
		if ps.Base == nil && hasHistory && ps.CwdFrom != "" && ps.CwdFrom != src.Paths.Cwd {
			lines = append(lines, evLine("cwd-changed", "The working directory changed from "+ps.CwdFrom+" to "+src.Paths.Cwd+". Relative paths and sh() now use "+src.Paths.Cwd+". Project skills and AGENTS.md of "+src.Paths.Cwd+" are already in your system prompt."))
		}
	} else {
		lines = diffBaseline(ps.Base, w.Base)
	}
	ps.Base, ps.CwdFrom, ps.PolicySig = w.Base, "", sig
	res.State, res.Event = ps, eventBlock(lines)
	return res
}
