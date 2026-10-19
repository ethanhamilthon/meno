package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
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
	Model  modelSig
}

// modelSig is the model selection the model has been told about.
type modelSig struct{ Provider, Model, Effort string }

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
	Model   modelSig // the session's model
	Effort  string   // the effort used by the request being prepared
	Routed  bool     // true when routing is enabled for this request
	// NoMCPWait takes the servers as they are instead of waiting for those still
	// starting. A prompt is built from them, so only that waits; the next look
	// reports a server that came up in the meantime as an event.
	NoMCPWait bool
}

// maxMCPWait is how long a prompt build waits for MCP servers that are still
// starting.
const maxMCPWait = 3 * time.Second

// gather reads AGENTS.md files, discovers skills and looks at the MCP servers.
// Unreadable AGENTS.md files are skipped and reported in warnings.
func (w worldSrc) gather(ctx context.Context) (world, []string) {
	var out world
	var warnings []string
	if !w.NoMCPWait {
		w.MCP.WaitReady(ctx, maxMCPWait)
	}
	effort := w.Effort
	if effort == "" {
		effort = w.Model.Effort
	}
	in := prompt.Input{Cwd: w.Paths.Cwd, Now: w.Now, OS: prompt.DescribeOS(), Shell: "/bin/sh",
		Provider: w.Model.Provider, Model: w.Model.Model, Effort: effort}
	b := &baseline{
		Cwd: w.Paths.Cwd, Skills: map[string]skillSig{}, Agents: map[string]fileSig{}, MCP: map[string]mcpSig{},
		Date: w.Now.Format("2006-01-02"), Model: w.Model,
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
		out = append(out, evLine("date-changed", "The date is now "+formatDay(cur.Date)+"."))
	}
	if old.Model != cur.Model && cur.Model.Model != "" {
		out = append(out, evLine("model-changed", "Your model changed from "+describeSig(old.Model)+" to "+describeSig(cur.Model)+". Earlier assistant messages in this conversation were written by the previous model; the Environment section of your system prompt still names it."))
	}
	if old.Cwd != cur.Cwd {
		out = append(out, evLine("cwd-changed", "The working directory changed from "+old.Cwd+" to "+cur.Cwd+". Relative paths and sh() now use "+cur.Cwd+". Project skills and AGENTS.md of "+cur.Cwd+" are not in your system prompt; see the other events and reread them."))
	}
	return out
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
			out = append(out, evLine("skills-added", fmt.Sprintf("Skill %q is now available: %s", name, n.Path)))
		case o != n:
			out = append(out, evLine("skills-updated", fmt.Sprintf("Skill %q was updated: reread %s before you use it.", name, n.Path)))
		}
	}
	for _, name := range sortedKeys(old) {
		if _, ok := cur[name]; !ok {
			out = append(out, evLine("skills-removed", fmt.Sprintf("Skill %q is no longer available.", name)))
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
		return evLine("agents-md-updated", title+" were added: read "+cur.Path+".")
	case !cur.Exists:
		return evLine("agents-md-updated", title+" were removed: "+old.Path+" is gone.")
	case old != cur:
		return evLine("agents-md-updated", title+" changed: reread "+cur.Path+".")
	}
	return ""
}
