package api

// ConfigLoginService is an OAuth provider the helper supports.
type ConfigLoginService struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type LoginServicesResponse struct {
	Services []ConfigLoginService `json:"services"`
}
type LoginStartRequest struct {
	Service string `json:"service"`
}
type LoginStartResponse struct {
	URL       string `json:"url"`
	State     string `json:"state"`
	UserCode  string `json:"user_code,omitempty"`
	Device    bool   `json:"device,omitempty"`
	ExpiresIn int64  `json:"expires_in"`
}
type LoginStatusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
type OAuthAccount struct {
	File     string `json:"file"`
	Provider string `json:"provider"`
	Service  string `json:"service"`
	Label    string `json:"label"`
	Plan     string `json:"plan,omitempty"`
	Disabled bool   `json:"disabled"`
	Failed   bool   `json:"failed,omitempty"`
	Note     string `json:"note,omitempty"`
}
type AccountsResponse struct {
	Accounts []OAuthAccount `json:"accounts"`
}
type AccountEnabledRequest struct {
	Enabled bool `json:"enabled"`
}
type LoginCallbackRequest struct {
	RedirectURL string `json:"redirect_url"`
}
