package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Agent Skills authoring limits (https://agentskills.io/specification).
const (
	maxSkillNameLen = 64
	maxSkillDescLen = 1024
)

// dirSkill reports whether path is a SKILL.md inside a skill directory (the
// standard layout) rather than a native single "name.md" file.
func dirSkill(path string) bool {
	return strings.EqualFold(filepath.Base(path), "SKILL.md")
}

// lintSkill returns authoring warnings. Loading stays lenient: meno still
// normalizes names and falls back to the first body line for a description.
func lintSkill(rawName, name, rawDesc, desc, fallback string, hasHeader, standard bool) []string {
	var w []string
	if standard && !hasHeader {
		w = append(w, "SKILL.md has no frontmatter")
	}
	if strings.TrimSpace(rawDesc) == "" {
		w = append(w, "missing description; using the first body line")
		if desc == "" {
			w[len(w)-1] = "missing description"
		}
	} else if utf8.RuneCountInString(rawDesc) > maxSkillDescLen {
		w = append(w, fmt.Sprintf("description is longer than %d characters", maxSkillDescLen))
	}
	if rawName = strings.TrimSpace(rawName); rawName != "" {
		if rawName != name {
			w = append(w, fmt.Sprintf("name %q is not a valid skill name; using %q", rawName, name))
		}
		if utf8.RuneCountInString(rawName) > maxSkillNameLen {
			w = append(w, fmt.Sprintf("name is longer than %d characters", maxSkillNameLen))
		}
		if standard && name != SkillName(fallback) {
			w = append(w, fmt.Sprintf("name %q does not match the directory %q", name, fallback))
		}
	}
	return w
}
