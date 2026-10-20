package proto

// ConfigModel is one provider model with catalog metadata.
type ConfigModel struct {
	Provider      string             `json:"provider"`
	ID            string             `json:"id"`
	Efforts       []string           `json:"efforts"`
	ContextWindow *int               `json:"contextWindow"`
	Images        *bool              `json:"images"`
	Prices        *ConfigModelPrices `json:"prices"`
	Stale         bool               `json:"stale"`
	Source        string             `json:"source"`
}
type ConfigModelPrices struct {
	Input            float64  `json:"input"`
	Output           float64  `json:"output"`
	CacheRead        *float64 `json:"cacheRead,omitempty"`
	CacheWrite       *float64 `json:"cacheWrite,omitempty"`
	PerMillionTokens bool     `json:"perMillionTokens"`
}
type ModelsRequest struct {
	Provider       string `json:"provider,omitempty"`
	Refresh        bool   `json:"refresh,omitempty"`
	RefreshCatalog bool   `json:"refresh_catalog,omitempty"`
}
type ModelsResponse struct {
	Models         []ConfigModel     `json:"models"`
	Errors         map[string]string `json:"errors,omitempty"`
	CatalogUpdated bool              `json:"catalogUpdated,omitempty"`
}
