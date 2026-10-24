package api

// ConfigModelSettings holds one provider's model overrides.
type ConfigModelSettings struct {
	Context    int      `json:"context,omitempty"`
	Input      *float64 `json:"input,omitempty"`
	Output     *float64 `json:"output,omitempty"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
	Vision     *bool    `json:"vision,omitempty"`
	Efforts    []string `json:"efforts,omitempty"`
}

// ConfigProvider describes a configured API provider for clients.
type ConfigProvider struct {
	Name        string                         `json:"name"`
	BaseURL     string                         `json:"base_url"`
	APIKey      string                         `json:"api_key,omitempty"`
	HasAPIKey   bool                           `json:"has_api_key,omitempty"`
	ClearAPIKey bool                           `json:"clear_api_key,omitempty"`
	Protocol    string                         `json:"protocol"`
	Disabled    bool                           `json:"disabled,omitempty"`
	Run         string                         `json:"run,omitempty"`
	Models      map[string]ConfigModelSettings `json:"models,omitempty"`
}

type ProvidersResponse struct {
	Providers []ConfigProvider `json:"providers"`
	Warnings  []string         `json:"warnings,omitempty"`
}
type ProviderRequest struct {
	Provider ConfigProvider `json:"provider"`
}
type ProviderResponse struct {
	Provider ConfigProvider `json:"provider"`
}
type ProviderEnabledRequest struct {
	Enabled bool `json:"enabled"`
}
type ProviderModelsRequest struct {
	Models map[string]ConfigModelSettings `json:"models"`
}
type AckResponse struct {
	OK bool `json:"ok"`
}
