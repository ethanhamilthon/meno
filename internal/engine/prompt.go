package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"meno/internal/config"
	"meno/internal/prompt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type modelSig struct{ Provider, Model, Effort string }
type baseline struct {
	cwd, date  string
	model      modelSig
	files      map[string]string
	skills     map[string]string
	skillPaths map[string]string
	tools      []string
}
type promptState struct {
	system string
	base   baseline
	policy string // the tool policy the frozen text was built for
}

// policyKey names the tool policy of a prompt: what it may and may not use.
func policyKey(in prompt.Input) string {
	return strings.Join(in.Allow, ",")
}

// prepare looks at the world and returns the system prompt for the next request,
// the event block describing what changed since the prompt was written ("" when
// nothing did), and the state to remember. The prompt text is frozen: it is
// rebuilt only at a reset point (a new session, a changed tool policy), so the
// provider cache keeps serving it; everything else reaches the model as events.
func (e *Engine) prepare(ctx context.Context, s *runState, effort string, first bool) (string, string, promptState) {
	_ = e.mcp.WaitReady(ctx, 3*time.Second)
	s.mu.Lock()
	cwd, provider, model, only, allow := s.cwd, s.session.Provider, s.session.Model, s.modes.Only, groupsOf(s)
	s.mu.Unlock()
	paths := e.paths
	paths.Cwd = cwd
	skills, _ := config.NewStore(paths).LoadSkills()
	in := prompt.Input{Cwd: cwd, Now: time.Now(), OS: prompt.DescribeOS(), Shell: "/bin/sh", Provider: provider, Model: model, Effort: effort, Only: only, Tool: "bash", Allow: allow}
	base := baseline{cwd: cwd, date: time.Now().Format("2006-01-02"), model: modelSig{provider, model, effort}, files: map[string]string{}, skills: map[string]string{}, skillPaths: map[string]string{}}
	for _, sk := range skills {
		if sk.Enabled {
			in.Skills = append(in.Skills, prompt.Skill{Name: sk.Name, Desc: sk.Desc, Path: sk.Path})
			base.skills[sk.Name] = hashFile(sk.Path)
			base.skillPaths[sk.Name] = sk.Path
		}
	}
	global, project := filepath.Join(e.home, "AGENTS.md"), filepath.Join(cwd, "AGENTS.md")
	in.GlobalAgents, _ = prompt.LoadInstructions(global)
	in.ProjectAgents, _ = prompt.LoadInstructions(project)
	base.files[global] = hashFile(global)
	base.files[project] = hashFile(project)
	for _, name := range e.mcp.Names() {
		for _, t := range e.mcp.ToolsOf(name) {
			in.Tools = append(in.Tools, prompt.Tool{Server: name, Name: t.Name, Desc: t.Description, Params: t.Signature()})
			base.tools = append(base.tools, name+"."+t.Name)
		}
	}
	sort.Strings(base.tools)
	s.mu.Lock()
	prev := s.prompt
	s.mu.Unlock()
	system := prompt.Build(in)
	policy := policyKey(in)
	if prev.system != "" && prev.policy == policy && !first {
		// Same tool policy: keep the frozen text and say what changed.
		return prev.system, eventBlock(diffBaseline(prev.base, base)), promptState{system: prev.system, base: base, policy: policy}
	}
	return system, "", promptState{system: system, base: base, policy: policy}
}
func hashFile(path string) string {
	f, e := os.Open(path)
	if e != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	_, _ = io.Copy(h, io.LimitReader(f, 4<<20))
	return hex.EncodeToString(h.Sum(nil)[:8])
}

func sameBaseline(a, b baseline) bool {
	if a.cwd != b.cwd || a.date != b.date || a.model != b.model || len(a.files) != len(b.files) || len(a.skills) != len(b.skills) || len(a.tools) != len(b.tools) {
		return false
	}
	for k, v := range a.files {
		if b.files[k] != v {
			return false
		}
	}
	for k, v := range a.skills {
		if b.skills[k] != v {
			return false
		}
	}
	for i := range a.tools {
		if a.tools[i] != b.tools[i] {
			return false
		}
	}
	return true
}
