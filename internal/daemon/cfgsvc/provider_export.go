package cfgsvc

import "meno/internal/llm"

// Providers is the full list the daemon works with, with API keys: saved
// providers, provider.json and the one derived from the OAuth accounts. The
// engine uses it to talk to the models; it never goes over the wire.
func (s *Service) Providers() []llm.Provider {
	all, err := s.providers()
	if err != nil {
		return nil
	}
	return all
}
