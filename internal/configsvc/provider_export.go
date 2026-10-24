package configsvc

import "meno/internal/llm"

// Providers is the list from provider.json with resolved API keys. The
// engine uses it to talk to the models; it never goes over the wire.
func (s *Service) Providers() []llm.Provider {
	all, err := s.providers()
	if err != nil {
		return nil
	}
	return all
}
