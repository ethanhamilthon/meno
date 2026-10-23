package engine

import "meno/internal/config"

// skillPaths lists the files of the enabled skills visible from cwd, manual-only
// ones included: a /skill:name selection reads them too.
func (e *Engine) skillPaths(cwd string) []string {
	paths := e.paths
	paths.Cwd = cwd
	list, _ := config.NewStore(paths).LoadSkills()
	var out []string
	for _, s := range list {
		if s.Enabled {
			out = append(out, s.Path)
		}
	}
	return out
}

// skillPathsOf is skillPaths for the current directory of a session.
func (e *Engine) skillPathsOf(s *runState) []string {
	s.mu.Lock()
	cwd := s.cwd
	s.mu.Unlock()
	return e.skillPaths(cwd)
}
