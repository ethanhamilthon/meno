package config

import (
	"os"
	"path/filepath"
)

type Paths struct {
	Home string
	Cwd  string
}

func DefaultPaths() (Paths, error) {
	home := os.Getenv("MENO_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, err
		}
		home = filepath.Join(h, ".meno")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return Paths{}, err
	}
	return Paths{Home: home, Cwd: cwd}, nil
}

func (p Paths) File(name string) string  { return filepath.Join(p.Home, name) }
func (p Paths) ProjectDir() string       { return filepath.Join(p.Cwd, ".meno") }
func (p Paths) SessionsDir() string      { return filepath.Join(p.Home, "sessions") }
func (p Paths) GlobalSkillsDir() string  { return filepath.Join(p.Home, "skills") }
func (p Paths) ProjectSkillsDir() string { return filepath.Join(p.ProjectDir(), "skills") }

// UserAgentsSkillsDir is ~/.agents/skills, the cross-client convention. It is
// found only when Home is the usual <user home>/.meno, so a custom MENO_HOME
// (and every test home) never reads the real user directory.
func (p Paths) UserAgentsSkillsDir() string {
	if filepath.Base(p.Home) != ".meno" {
		return ""
	}
	return filepath.Join(filepath.Dir(p.Home), ".agents", "skills")
}

// ProjectAgentsSkillsDir is <project>/.agents/skills.
func (p Paths) ProjectAgentsSkillsDir() string {
	if p.Cwd == "" {
		return ""
	}
	return filepath.Join(p.Cwd, ".agents", "skills")
}

// SkillDirs lists the skill directories from lowest to highest precedence:
// the cross-client user directory, the meno global one, the cross-client
// project directory, then the meno project one.
func (p Paths) SkillDirs() []SkillDir {
	return []SkillDir{
		{p.UserAgentsSkillsDir(), "global"},
		{p.GlobalSkillsDir(), "global"},
		{p.ProjectAgentsSkillsDir(), "project"},
		{p.ProjectSkillsDir(), "project"},
	}
}
