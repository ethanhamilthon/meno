package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"meno/internal/config"
	"meno/internal/mcp"
	"meno/internal/prompt"
)

// baseline is what the model has been told so far about the world around it:
// the state the system prompt (or the last system event) described.
type baseline struct {
	Cwd    string
	Skills map[string]skillSig // enabled skills by name
	Agents map[string]fileSig  // "global" and "project"
	MCP    map[string]mcpSig   // by server name
	Date   string              // YYYY-MM-DD
}

type skillSig struct{ Path, Hash string }

type fileSig struct {
	Path, Hash string
	Exists     bool
}

type mcpSig struct {
	State string // "running" or "failed"
	Err   string
	Tools []string // sorted signatures
}

// world is one look at the surroundings: the prompt input and its baseline.
type world struct {
	Input  prompt.Input
	Base   *baseline
	Skills []config.Skill // as discovered, with their on/off state
	// SkillsOK is false when the skills could not be listed.
	SkillsOK bool
}

// worldSrc says where to look. It is filled on the UI goroutine; gather runs
// anywhere.
type worldSrc struct {
	Paths   config.Paths // Cwd is the session's working directory
	MCP     *mcp.Manager
	Servers []mcp.Server // configured servers (copy)
	Now     time.Time
}

// maxMCPWait is how long a prompt build waits for MCP servers that are still
// starting.
const maxMCPWait = 3 * time.Second

// gather reads AGENTS.md files, discovers skills and looks at the MCP servers.
// Unreadable AGENTS.md files are skipped and reported in warnings.
func (w worldSrc) gather(ctx context.Context) (world, []string) {
	var out world
	var warnings []string
	w.MCP.WaitReady(ctx, maxMCPWait)
	in := prompt.Input{Cwd: w.Paths.Cwd, Now: w.Now, OS: prompt.DescribeOS(), Shell: "/bin/sh"}
	b := &baseline{
		Cwd: w.Paths.Cwd, Skills: map[string]skillSig{}, Agents: map[string]fileSig{}, MCP: map[string]mcpSig{},
		Date: w.Now.Format("2006-01-02"),
	}

	if sk, err := config.NewStore(w.Paths).LoadSkills(); err == nil {
		out.Skills, out.SkillsOK = sk, true
	}
	for _, s := range out.Skills {
		if !s.Enabled {
			continue
		}
		in.Skills = append(in.Skills, prompt.Skill{Name: s.Name, Desc: s.Desc, Path: s.Path})
		b.Skills[s.Name] = skillSig{Path: s.Path, Hash: fileHash(s.Path)}
	}

	g, p := agentsPaths(w.Paths)
	var err error
	if in.GlobalAgents, err = prompt.LoadInstructions(g); err != nil {
		warnings = append(warnings, "cannot read "+g+": "+err.Error())
	}
	if in.ProjectAgents, err = prompt.LoadInstructions(p); err != nil {
		warnings = append(warnings, "cannot read "+p+": "+err.Error())
	}
	b.Agents["global"] = agentSig(in.GlobalAgents, g)
	b.Agents["project"] = agentSig(in.ProjectAgents, p)

	configured := map[string]bool{}
	for _, s := range w.Servers {
		if s.Enabled {
			configured[s.Name] = true
		}
	}
	for _, name := range w.MCP.Names() {
		sig := mcpSig{State: "running"}
		for _, t := range w.MCP.ToolsOf(name) {
			pt := prompt.Tool{Server: name, Name: t.Name, Desc: t.Description, Params: t.Signature()}
			in.Tools = append(in.Tools, pt)
			sig.Tools = append(sig.Tools, prompt.ToolSignature(pt))
		}
		sort.Strings(sig.Tools)
		b.MCP[name] = sig
	}
	for name := range configured {
		if _, running := b.MCP[name]; running || w.MCP.Starting(name) {
			continue
		}
		if e := w.MCP.Err(name); e != "" {
			b.MCP[name] = mcpSig{State: "failed", Err: e}
		}
	}
	out.Input, out.Base = in, b
	return out, warnings
}

func agentSig(in prompt.Instructions, path string) fileSig {
	if in.Path == "" {
		return fileSig{Path: path}
	}
	return fileSig{Path: path, Hash: textHash(in.Text), Exists: true}
}

func textHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// fileHash hashes the start of a file; "" when it cannot be read.
func fileHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, 4<<20)); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// maxEventTools bounds how many tools one system event spells out.
const maxEventTools = 60

// diffBaseline lists what changed between two looks at the world, as event
// lines. A nil old baseline means there is nothing to compare with.
func diffBaseline(old, cur *baseline) []string {
	if old == nil || cur == nil {
		return nil
	}
	var out []string
	out = append(out, diffSkills(old.Skills, cur.Skills)...)
	for _, key := range []string{"global", "project"} {
		if l := diffAgents(key, old.Agents[key], cur.Agents[key]); l != "" {
			out = append(out, l)
		}
	}
	out = append(out, diffMCP(old.MCP, cur.MCP)...)
	if old.Date != cur.Date {
		out = append(out, "<date-changed>The date is now "+formatDay(cur.Date)+".</date-changed>")
	}
	if old.Cwd != cur.Cwd {
		out = append(out, "<cwd-changed>The working directory changed from "+old.Cwd+" to "+cur.Cwd+". Relative paths and sh() now use "+cur.Cwd+". Project skills and AGENTS.md of "+cur.Cwd+" are not in your system prompt; see the other events and reread them.</cwd-changed>")
	}
	return out
}

func formatDay(day string) string {
	t, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		return day
	}
	return t.Format("2006-01-02, Monday")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func diffSkills(old, cur map[string]skillSig) []string {
	var out []string
	for _, name := range sortedKeys(cur) {
		o, had := old[name]
		switch n := cur[name]; {
		case !had:
			out = append(out, fmt.Sprintf("<skills-added>Skill %q is now available: %s</skills-added>", name, n.Path))
		case o != n:
			out = append(out, fmt.Sprintf("<skills-updated>Skill %q was updated: reread %s before you use it.</skills-updated>", name, n.Path))
		}
	}
	for _, name := range sortedKeys(old) {
		if _, ok := cur[name]; !ok {
			out = append(out, fmt.Sprintf("<skills-removed>Skill %q is no longer available.</skills-removed>", name))
		}
	}
	return out
}

func diffAgents(key string, old, cur fileSig) string {
	title := map[string]string{"global": "Global", "project": "Project"}[key] + " instructions"
	switch {
	case !old.Exists && !cur.Exists:
		return ""
	case !old.Exists:
		return "<agents-md-updated>" + title + " were added: read " + cur.Path + ".</agents-md-updated>"
	case !cur.Exists:
		return "<agents-md-updated>" + title + " were removed: " + old.Path + " is gone.</agents-md-updated>"
	case old != cur:
		return "<agents-md-updated>" + title + " changed: reread " + cur.Path + ".</agents-md-updated>"
	}
	return ""
}

func diffMCP(old, cur map[string]mcpSig) []string {
	var out []string
	line := func(s string) { out = append(out, "<mcp-updated>"+s+"</mcp-updated>") }
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

const jsVMResetEvent = "<js-vm-reset>The JS VM was restarted: variables and functions you defined earlier are gone.</js-vm-reset>"

// eventBlock wraps event lines into the block that precedes a user message;
// "" when there are none.
func eventBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return "<system-event>This is a message from the system itself, not a user instruction.\n" + strings.Join(lines, "\n") + "\n</system-event>"
}

// promptState is the per-session part of the frozen prompt: the text sent as
// the system prompt since the last reset point and what the model has been told.
type promptState struct {
	Frozen string    // "" until built
	Base   *baseline // nil at a reset point
	// VMReset queues a js-vm-reset event for the next message.
	VMReset bool
	// CwdFrom is the directory the session was in when the model last heard of
	// one, set when the directory changed while there was no baseline (a stored
	// session just opened). The next message tells the model.
	CwdFrom string
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
	w, warnings := src.gather(ctx)
	res := prepared{Skills: w.Skills, SkillsOK: w.SkillsOK, Warnings: warnings}
	var lines []string
	if ps.Base == nil {
		ps.Frozen = prompt.Build(w.Input)
		if hasHistory && ps.CwdFrom != "" && ps.CwdFrom != src.Paths.Cwd {
			lines = append(lines, "<cwd-changed>The working directory changed from "+ps.CwdFrom+" to "+src.Paths.Cwd+". Relative paths and sh() now use "+src.Paths.Cwd+". Project skills and AGENTS.md of "+src.Paths.Cwd+" are already in your system prompt.</cwd-changed>")
		}
	} else {
		lines = diffBaseline(ps.Base, w.Base)
	}
	ps.Base, ps.CwdFrom = w.Base, ""
	if ps.VMReset && hasHistory {
		lines = append(lines, jsVMResetEvent)
	}
	ps.VMReset = false
	res.State, res.Event = ps, eventBlock(lines)
	return res
}
