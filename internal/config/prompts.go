package config

import (
	"errors"
	"meno/internal/store"
	"regexp"
	"strings"
)

type Prompt struct {
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	Template string `json:"template"`
}

var promptName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

func DefaultPrompts() []Prompt {
	return []Prompt{
		{Name: "rate-limit", Scope: "project", Template: "добавь rateLimit middleware (20 req/min) и проверь роуты"},
		{Name: "security-audit", Scope: "project", Template: "проверь роуты на auth bypass, injection и валидацию"},
		{Name: "refactor-clean", Scope: "global", Template: "упрости код без изменения внешнего поведения"},
		{Name: "unit-tests", Scope: "global", Template: "напиши тесты на граничные случаи и ошибки"},
		{Name: "perf-check", Scope: "global", Template: "найди узкие места и лишние аллокации в горячем пути"},
	}
}

func ValidatePrompt(p Prompt) error {
	name := strings.TrimPrefix(p.Name, "#")
	if !promptName.MatchString(name) {
		return errors.New("name: a-z, 0-9, - (max 40), must start with letter or digit")
	}
	if strings.TrimSpace(p.Template) == "" {
		return errors.New("template is empty")
	}
	return nil
}

func FindPrompt(ps []Prompt, name string) (Prompt, bool) {
	name = strings.TrimPrefix(name, "#")
	for _, p := range ps {
		if p.Name == name {
			return p, true
		}
	}
	return Prompt{}, false
}

func (s *Store) LoadPrompts() ([]Prompt, error) {
	var ps []Prompt
	err := store.LoadJSON(s.P.File("prompts.json"), &ps)
	if ps == nil && err == nil {
		ps = DefaultPrompts()
	}
	return ps, err
}

func (s *Store) SavePrompts(ps []Prompt) error {
	return store.SaveJSON(s.P.File("prompts.json"), ps)
}
