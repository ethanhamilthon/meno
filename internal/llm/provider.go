package llm

import (
	"errors"
	"strings"
)

const (
	ProtoResponses = "responses"
	ProtoChat      = "chat completions"
)

// SourceFile marks a provider read from provider.json: it is read only and
// never saved to the database.
const SourceFile = "file"

// SourceOAuth marks the provider meno derives from the OAuth accounts of the
// local helper. It is rebuilt at every start and never saved.
const SourceOAuth = "oauth"

// OAuthName is the name of that provider.
const OAuthName = "oauth"

type Provider struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Protocol string `json:"protocol"`
	Source   string `json:"-"`
	// Disabled keeps the provider in the list but out of the model list and out of use.
	Disabled bool `json:"disabled,omitempty"`
	// Run is a shell command meno starts at launch (for a local proxy) and stops
	// at exit. It is not waited for: a request that fails while the proxy is
	// starting is retried.
	Run string `json:"run,omitempty"`
	// Models holds the user's own settings for single models, by model id.
	Models map[string]ModelSettings `json:"models,omitempty"`
}

// Active reports whether the provider is switched on.
func (p Provider) Active() bool { return !p.Disabled }

// Managed reports whether meno derives the provider from OAuth accounts.
func (p Provider) Managed() bool { return p.Source == SourceOAuth }

// FromFile reports whether the provider is defined in provider.json.
func (p Provider) FromFile() bool { return p.Source == SourceFile }

func Validate(p Provider) error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("name is required")
	}
	if strings.ContainsAny(p.Name, " \t/") {
		return errors.New("name must not contain spaces or slashes")
	}
	u := strings.TrimSpace(p.BaseURL)
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return errors.New("base url must start with http:// or https://")
	}
	if p.Protocol != ProtoResponses && p.Protocol != ProtoChat {
		return errors.New("protocol must be responses or chat completions")
	}
	return ValidateModels(p.Models)
}

func Mask(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 4 {
		return strings.Repeat("•", len(key))
	}
	return strings.Repeat("•", 20) + key[len(key)-2:]
}

func (p Provider) Find(ps []Provider) (Provider, bool) {
	for _, x := range ps {
		if x.Name == p.Name {
			return x, true
		}
	}
	return Provider{}, false
}
