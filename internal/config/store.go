package config

import (
	"meno/internal/llm"
)

type Store struct {
	P Paths
}

func NewStore(p Paths) *Store { return &Store{P: p} }

func (s *Store) LoadSettings() (Settings, error) {
	st := DefaultSettings()
	err := LoadJSON(s.P.File("settings.json"), &st)
	return st, err
}

func (s *Store) SaveSettings(st Settings) error {
	return SaveJSON(s.P.File("settings.json"), st)
}

func (s *Store) LoadProviders() ([]llm.Provider, error) {
	var ps []llm.Provider
	err := LoadJSON(s.P.File("providers.json"), &ps)
	if len(ps) == 0 {
		ps = llm.DefaultProviders()
	}
	return ps, err
}

func (s *Store) SaveProviders(ps []llm.Provider) error {
	return SaveJSON(s.P.File("providers.json"), ps)
}
