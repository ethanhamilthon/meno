package api

import (
	"encoding/json"
	"testing"
)

func TestConfigWireGoldens(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"settings_request", SettingsRequest{Settings: ConfigSettings{Effort: "high"}}, `{"settings":{"provider":"","model":"","effort":"high","tool":"","view":"","turn_summary":"","notify":"","notify_command":"","notify_sound":"","notify_volume":0,"compact_percent":0,"compact_keep":0,"compact_summary_tokens":0,"elide_percent":0,"system_prompt":"","max_spawn_depth":0,"max_workers":0}}`},
		{"provider_request", ProviderRequest{Provider: ConfigProvider{Name: "p", BaseURL: "https://x", Protocol: "chat completions", APIKey: "k"}}, `{"provider":{"name":"p","base_url":"https://x","api_key":"k","protocol":"chat completions"}}`},
		{"provider_key_state", ConfigProvider{Name: "p", HasAPIKey: true}, `{"name":"p","base_url":"","has_api_key":true,"protocol":""}`},
		{"provider_enabled", ProviderEnabledRequest{Enabled: true}, "{\"enabled\":true}"},
		{"skill_enabled", SkillEnabledRequest{Enabled: false}, "{\"enabled\":false}"},
		{"mcp_request", McpServerRequest{Server: ConfigMCPServer{Name: "git", Transport: "stdio", Command: "git", Enabled: true}}, `{"server":{"name":"git","transport":"stdio","command":"git","enabled":true,"source":"","running":false,"starting":false}}`},
		{"mcp_server", ConfigMCPServer{Name: "git", Transport: "stdio", Command: "git", Enabled: true}, "{\"name\":\"git\",\"transport\":\"stdio\",\"command\":\"git\",\"enabled\":true,\"source\":\"\",\"running\":false,\"starting\":false}"},
		{"models_request", ModelsRequest{Provider: "p", Refresh: true, RefreshCatalog: true}, `{"provider":"p","refresh":true,"refresh_catalog":true}`},
		{"models", ModelsResponse{Models: []ConfigModel{{Provider: "p", ID: "m", Source: "catalog", Efforts: []string{}}}}, "{\"models\":[{\"provider\":\"p\",\"id\":\"m\",\"efforts\":[],\"contextWindow\":null,\"images\":null,\"prices\":null,\"stale\":false,\"source\":\"catalog\"}]}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, e := json.Marshal(tc.value)
			if e != nil {
				t.Fatal(e)
			}
			if string(b) != tc.want {
				t.Fatalf("got %s want %s", b, tc.want)
			}
		})
	}
}
