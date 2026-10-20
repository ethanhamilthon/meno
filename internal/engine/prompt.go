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
	cwd, date string
	model     modelSig
	files     map[string]string
	skills    map[string]string
	tools     []string
}
type promptState struct {
	system string
	base   baseline
}

func (e *Engine) prepare(ctx context.Context, s *runState, effort string, first bool) (string, string, promptState) {
	_ = e.mcp.WaitReady(ctx, 3*time.Second)
	paths := e.paths
	paths.Cwd = s.cwd
	skills, _ := config.NewStore(paths).LoadSkills()
	in := prompt.Input{Cwd: s.cwd, Now: time.Now(), OS: prompt.DescribeOS(), Shell: "/bin/sh", Provider: s.session.Provider, Model: s.session.Model, Effort: effort, Only: s.modes.Only}
	base := baseline{cwd: s.cwd, date: time.Now().Format("2006-01-02"), model: modelSig{s.session.Provider, s.session.Model, effort}, files: map[string]string{}, skills: map[string]string{}}
	for _, sk := range skills {
		if sk.Enabled {
			in.Skills = append(in.Skills, prompt.Skill{Name: sk.Name, Desc: sk.Desc, Path: sk.Path})
			base.skills[sk.Name] = hashFile(sk.Path)
		}
	}
	global, project := filepath.Join(e.home, "AGENTS.md"), filepath.Join(s.cwd, "AGENTS.md")
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
	system := prompt.Build(in)
	diff := ""
	if !first {
		if s.prompt.system == "" {
		} else if !sameBaseline(s.prompt.base, base) {
			diff = `<system-event kind="world-changed">The working directory, instructions, skills, model, or available tools changed. Re-read the relevant instructions and environment before continuing.</system-event>`
		}
	}
	return system, diff, promptState{system: system, base: base}
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
