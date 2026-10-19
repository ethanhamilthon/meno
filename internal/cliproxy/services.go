package cliproxy

// Service is one account type the helper can log in to.
type Service struct {
	ID    string   // the provider name of the login endpoint
	Label string   // what the user sees
	Files []string // provider names that auth files of this service carry
}

// Services lists what :login offers, in the order of the list.
var Services = []Service{
	{ID: "claude", Label: "Claude", Files: []string{"claude", "anthropic"}},
	{ID: "codex", Label: "ChatGPT / Codex", Files: []string{"codex"}},
	{ID: "antigravity", Label: "Antigravity (Google)", Files: []string{"antigravity"}},
	{ID: "xai", Label: "xAI (Grok)", Files: []string{"xai"}},
	{ID: "meta", Label: "Meta", Files: []string{"meta"}},
	{ID: "kimi", Label: "Kimi", Files: []string{"kimi", "kimi-ai"}},
}

// ServiceOf finds the service that an auth file provider name belongs to.
func ServiceOf(fileProvider string) (Service, bool) {
	for _, s := range Services {
		for _, f := range s.Files {
			if f == fileProvider {
				return s, true
			}
		}
	}
	return Service{}, false
}

// ServiceByID finds a service by its login id.
func ServiceByID(id string) (Service, bool) {
	for _, s := range Services {
		if s.ID == id {
			return s, true
		}
	}
	return Service{}, false
}
