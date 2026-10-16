package config

import (
	"meno/internal/store"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Skill struct {
	Name    string `json:"name"`
	Scope   string `json:"scope"`
	Desc    string `json:"desc"`
	Enabled bool   `json:"enabled"`
}

func DefaultSkills() []Skill {
	return []Skill{
		{Name: "api-security", Scope: "project", Desc: "проверка auth, rateLimit, JWT и валидации входных схем", Enabled: true},
		{Name: "express-routes", Scope: "project", Desc: "правила для express-роутов", Enabled: true},
		{Name: "postgres-migrations", Scope: "global", Desc: "миграции postgres"},
		{Name: "playwright-e2e", Scope: "global", Desc: "e2e тесты playwright"},
		{Name: "docker-deploy", Scope: "builtin", Desc: "деплой через docker"},
	}
}

func (s *Store) LoadSkills() ([]Skill, error) {
	var sk []Skill
	err := store.LoadJSON(s.P.File("skills.json"), &sk)
	if sk == nil && err == nil {
		sk = DefaultSkills()
	}
	return sk, err
}

func (s *Store) SaveSkills(sk []Skill) error {
	return store.SaveJSON(s.P.File("skills.json"), sk)
}

func DiscoverSkillFiles(dirs ...string) []string {
	var out []string
	for _, d := range dirs {
		ents, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			out = append(out, filepath.Join(d, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}

func FilterSkills(sk []Skill, q string) []Skill {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return sk
	}
	var out []Skill
	for _, s := range sk {
		if strings.Contains(strings.ToLower(s.Name+" "+s.Desc), q) {
			out = append(out, s)
		}
	}
	return out
}
