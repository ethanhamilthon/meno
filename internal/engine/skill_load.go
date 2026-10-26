package engine

import (
	"os"
	"path/filepath"
	"strings"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/content"
)

// skillTurns loads the skills a user message names with /skill:name, each once.
// On the first message of a new session the skills marked auto-load are added,
// in front of the text, as ordinary skill references. A skill that is disabled,
// missing or unreadable is skipped. It returns the text to store and the turns
// that carry the skill texts.
func (e *Engine) skillTurns(s *runState, text string, first bool) (string, []agent.Turn) {
	skills := e.skillsOf(s)
	byName := map[string]config.Skill{}
	for _, k := range skills {
		if k.Enabled {
			byName[k.Name] = k
		}
	}
	var names []string
	seen := map[string]bool{}
	add := func(name string) {
		if _, ok := byName[name]; ok && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, seg := range content.Split(text, nil) {
		if seg.Obj != nil && seg.Obj.Kind == content.Skill {
			add(seg.Obj.Ref)
		}
	}
	if first {
		var prefix []string
		for _, k := range skills {
			if k.Auto && k.Enabled && !seen[k.Name] {
				add(k.Name)
				prefix = append(prefix, content.Object{Kind: content.Skill, Ref: k.Name}.Tag())
			}
		}
		if len(prefix) > 0 {
			text = strings.Join(prefix, " ") + " " + text
		}
	}
	var turns []agent.Turn
	for _, name := range names {
		k := byName[name]
		raw, err := os.ReadFile(k.Path)
		if err != nil {
			continue
		}
		_, body, _ := config.ParseFrontmatterDiag(string(raw))
		path, err := filepath.Abs(k.Path)
		if err != nil {
			path = k.Path
		}
		turns = append(turns, agent.SkillTurn(name, path, body))
	}
	return text, turns
}

// skillsOf lists the skills seen from the directory of a session. The caller
// holds s.mu.
func (e *Engine) skillsOf(s *runState) []config.Skill {
	paths := e.paths
	paths.Cwd = s.cwd
	list, _ := config.NewStore(paths).LoadSkills()
	return list
}
