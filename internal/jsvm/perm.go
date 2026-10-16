package jsvm

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

const (
	ModeAuto = "auto"
	ModeAsk  = "ask"
)

type Gate struct {
	Root    string
	Approve func(action, target string) bool

	mu   sync.RWMutex
	mode string
}

func NewGate(root, mode string, approve func(action, target string) bool) *Gate {
	return &Gate{Root: root, mode: mode, Approve: approve}
}

func (g *Gate) SetMode(m string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.mode = m
}

func (g *Gate) Mode() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.mode
}

func (g *Gate) CheckPath(action, target string) error {
	if err := g.inRoot(target); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return g.approve(action, target)
}

func (g *Gate) CheckCommand(cmd string) error {
	return g.approve("sh", cmd)
}

func (g *Gate) approve(action, target string) error {
	if g.Mode() != ModeAsk {
		return nil
	}
	if g.Approve == nil || !g.Approve(action, target) {
		return fmt.Errorf("%s %s: denied by user", action, target)
	}
	return nil
}

func (g *Gate) inRoot(target string) error {
	root, err := filepath.Abs(g.Root)
	if err != nil {
		return err
	}
	p := target
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	p = filepath.Clean(p)
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is outside project root", target)
	}
	return nil
}
