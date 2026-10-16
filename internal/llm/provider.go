package llm

import (
	"errors"
	"strings"
)

const (
	ProtoResponses = "responses"
	ProtoChat      = "chat completions"
)

type Provider struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Protocol string `json:"protocol"`
}

func DefaultProviders() []Provider {
	return []Provider{
		{Name: "openai", BaseURL: "https://api.openai.com/v1", Protocol: ProtoResponses},
		{Name: "anthropic", BaseURL: "https://api.anthropic.com/v1", Protocol: ProtoChat},
		{Name: "openrouter", BaseURL: "https://openrouter.ai/api/v1", Protocol: ProtoChat},
		{Name: "local", BaseURL: "http://localhost:11434/v1", Protocol: ProtoChat},
	}
}

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
	return nil
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
