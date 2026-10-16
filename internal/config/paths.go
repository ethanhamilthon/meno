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
