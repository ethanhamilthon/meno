package configsvc

import (
	"os"

	"meno/internal/config"
	"meno/internal/llm"
)

func (s *Service) providers() ([]llm.Provider, error) {
	own, err := s.store.LoadProviders()
	if err != nil {
		return nil, err
	}
	files, _ := config.LoadProviderFile(s.paths.ProviderFile(), os.Getenv)
	stamp := s.stamp(s.paths.ProviderFile())
	if s.updateStamp(s.paths.ProviderFile(), stamp) {
		s.changed("providers_changed")
	}
	all, _ := config.MergeProviders(own, files)
	st := s.oauth.State()
	if st.Accounts > 0 && st.HasAccounts {
		all = append(all, llm.Provider{Name: llm.OAuthName, BaseURL: s.oauth.APIURL(), APIKey: s.oauth.APIKey(), Protocol: llm.ProtoChat, Run: s.oauth.Command(), Source: llm.SourceOAuth})
	}
	return all, nil
}
