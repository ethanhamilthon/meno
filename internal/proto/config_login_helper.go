package proto

// OAuthHelperResponse reports the daemon-owned helper process state.
type OAuthHelperResponse struct {
	Installed   bool   `json:"installed"`
	Running     bool   `json:"running"`
	Ready       bool   `json:"ready"`
	Version     string `json:"version"`
	Pending     string `json:"pending,omitempty"`
	Port        int    `json:"port"`
	Accounts    int    `json:"accounts"`
	HasAccounts bool   `json:"has_accounts"`
}
