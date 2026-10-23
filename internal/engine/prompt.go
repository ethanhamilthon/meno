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
}

// prepare looks at the world and returns the system prompt for the next request,
// the event block describing what changed since the prompt was written ("" when
// nothing did), and the state to remember. The prompt text is frozen: it is
// rebuilt only at a reset point (a new session, a forced rebuild), so the
// provider cache keeps serving it; everything else reaches the model as events.
func (e *Engine) prepare(ctx context.Context, s *runState, effort string, first bool) (string, string, promptState) {
	_ = e.mcp.WaitReady(ctx, 3*time.Second)
	s.mu.Lock()
	cwd, provider, model, identity := s.cwd, s.session.Provider, s.session.Model, s.session.SystemPrompt
	s.mu.Unlock()
	paths := e.paths
	paths.Cwd = cwd
	skills, _ := config.NewStore(paths).LoadSkills()
	in := prompt.Input{Identity: identity, Cwd: cwd, Now: time.Now(), OS: prompt.DescribeOS(), Shell: "/bin/sh", Provider: provider, Model: model, Effort: effort}
	base := baseline{cwd: cwd, date: time.Now().Format("2006-01-02"), model: modelSig{provider, model, effort}, files: map[string]string{}, skills: map[string]string{}, skillPaths: map[string]string{}}
	for _, sk := range skills {
		if sk.Enabled && !sk.ManualOnly { // manual-only skills are never offered to the model
			in.Skills = append(in.Skills, prompt.Skill{Name: sk.Name, Desc: sk.Desc, Path: sk.Path})
			base.skills[sk.Name] = hashFile(sk.Path)
			base.skillPaths[sk.Name] = sk.Path
		}
	}
	project := filepath.Join(cwd, "AGENTS.md")
	in.ProjectAgents, _ = prompt.LoadInstructions(project)
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
	if prev.system != "" && !first {
		// Keep the frozen text and say what changed.
		return prev.system, eventBlock(diffBaseline(prev.base, base)), promptState{system: prev.system, base: base}
	}
	return system, "", promptState{system: system, base: base}
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
