package configsvc

import (
	"os"

	"meno/internal/config"
	"meno/internal/llm"
)

// providers reads provider.json and resolves env:NAME API keys.
func (s *Service) providers() ([]llm.Provider, error) {
	all, _, err := s.providerDefinitions()
	return all, err
}

// providerDefinitions also returns what is wrong with provider.json. A file that
// is not valid JSON gives no saved providers and one warning: the daemon and the
// dialogs keep working, and a save is refused until the file is fixed.
func (s *Service) providerDefinitions() ([]llm.Provider, []string, error) {
	own, warnings, err := s.store.LoadProvidersFile()
	if err != nil {
		own, warnings = nil, []string{err.Error()}
	}
	stamp := s.stamp(s.paths.ProviderFile())
	if s.updateStamp(s.paths.ProviderFile(), stamp) {
		s.changed("providers_changed")
	}
	all := config.ResolveKeys(own, os.Getenv)
	return all, warnings, nil
}
