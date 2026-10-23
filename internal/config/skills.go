package config

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Skill is an instruction file the agent can read when a task matches it. It is
// found on disk; only the on/off switch is stored by meno.
type Skill struct {
	Name    string `json:"name"`
	Scope   string `json:"scope"` // "project" or "global"
	Desc    string `json:"desc"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
	// ManualOnly is the skill's `disable-model-invocation: true`: the model is
	// not told about it; only an explicit /skill:name selection uses it.
	ManualOnly bool `json:"manual_only,omitempty"`
	// Warnings are authoring problems found while loading (see skills_diag.go).
	// A skill with warnings is still loaded.
	Warnings []string `json:"warnings,omitempty"`
}

// maxSkillHead bounds how much of a skill file is read to find its description.
const maxSkillHead = 64 << 10

var skillNameRe = regexp.MustCompile(`[^a-z0-9_-]+`)

// SkillName turns a file or directory name into a name usable as /skill:<name>.
func SkillName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = skillNameRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-_")
}

// firstLine is the first line of body that is neither blank nor a heading.
func firstLine(body string) string {
	for _, l := range strings.Split(body, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			return l
		}
	}
	return ""
}

// readSkill reads one skill file. fallback names the skill when its header does not.
func readSkill(path, fallback, scope string) (Skill, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Skill{}, false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxSkillHead))
	if err != nil {
		return Skill{}, false
	}
	meta, body, warn := ParseFrontmatterDiag(string(b))
	var warnings []string
	if warn != "" {
		warnings = append(warnings, warn)
	}
	rawName := meta["name"]
	name := SkillName(rawName)
	if name == "" {
		name = SkillName(fallback)
	}
	if name == "" {
		return Skill{}, false
	}
	desc := strings.Join(strings.Fields(meta["description"]), " ")
	if desc == "" {
		desc = strings.Join(strings.Fields(firstLine(body)), " ")
	}
	manual := strings.EqualFold(strings.TrimSpace(meta["disable-model-invocation"]), "true")
	warnings = append(warnings, lintSkill(rawName, name, meta["description"], desc, fallback, meta != nil, dirSkill(path))...)
	return Skill{Name: name, Scope: scope, Desc: desc, Path: path, ManualOnly: manual, Warnings: warnings}, true
}

// scanSkillDir finds "name.md" files and "name/SKILL.md" directories.
func scanSkillDir(dir, scope string) []Skill {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Skill
	for _, e := range ents {
		path := filepath.Join(dir, e.Name())
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(path); err == nil {
				isDir = st.IsDir()
			}
		}
		var s Skill
		var ok bool
		switch {
		case isDir:
			for _, n := range []string{"SKILL.md", "skill.md"} {
				if s, ok = readSkill(filepath.Join(path, n), e.Name(), scope); ok {
					break
				}
			}
		case strings.HasSuffix(strings.ToLower(e.Name()), ".md") && !strings.EqualFold(e.Name(), "README.md"):
			s, ok = readSkill(path, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())), scope)
		}
		if ok {
			out = append(out, s)
		}
	}
	return out
}

// DiscoverSkills lists the skills of the project directory and the global one,
// sorted by name. A project skill replaces a global skill with the same name.
func DiscoverSkills(globalDir, projectDir string) []Skill {
	return DiscoverSkillDirs([]SkillDir{{globalDir, "global"}, {projectDir, "project"}})
}

// SkillDir is one directory to scan for skills and the scope its skills get.
type SkillDir struct {
	Dir, Scope string
}

// DiscoverSkillDirs scans the directories in order, sorted by name. A skill
// from a later directory replaces one with the same name from an earlier one.
func DiscoverSkillDirs(dirs []SkillDir) []Skill {
	byName := map[string]Skill{}
	for _, d := range dirs {
		if d.Dir == "" {
			continue
		}
		for _, s := range scanSkillDir(d.Dir, d.Scope) {
			byName[s.Name] = s
		}
	}
	out := make([]Skill, 0, len(byName))
	for _, s := range byName {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// LoadSkills discovers the skills on disk and applies the saved on/off state.
func (s *Store) LoadSkills() ([]Skill, error) {
	d, err := s.DB()
	if err != nil {
		return nil, err
	}
	rows, err := d.Query("SELECT name FROM skills_state WHERE enabled = 0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	off := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		off[n] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	list := DiscoverSkillDirs(s.P.SkillDirs())
	for i := range list {
		list[i].Enabled = !off[list[i].Name]
	}
	return list, nil
}

// SaveSkills stores the on/off state of the given skills. Skills that are not
// on disk right now keep their saved state.
func (s *Store) SaveSkills(sk []Skill) error {
	return s.tx(func(t *sql.Tx) error {
		for _, k := range sk {
			if _, err := t.Exec(`INSERT INTO skills_state(name,enabled) VALUES(?,?)
				ON CONFLICT(name) DO UPDATE SET enabled=excluded.enabled`, k.Name, b2i(k.Enabled)); err != nil {
				return err
			}
		}
		return nil
	})
}
