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
		{"settings_request", SettingsRequest{Settings: ConfigSettings{Effort: "high"}}, `{"settings":{"provider":"","model":"","effort":"high","tool":"","bash_code":"","bash_lines":0,"bash_output":"","bash_output_lines":0,"bash_timing":"","turn_summary":"","thinking":"","thinking_lines":0,"theme":"","notify":"","notify_command":"","notify_sound":"","notify_volume":0,"compact_percent":0,"compact_keep":0,"compact_summary_tokens":0,"images_protocol":"","web_search":"","web_allow_local":false}}`},
		{"provider_request", ProviderRequest{Provider: ConfigProvider{Name: "p", BaseURL: "https://x", Protocol: "chat completions", APIKey: "k"}}, `{"provider":{"name":"p","base_url":"https://x","api_key":"k","protocol":"chat completions","source":""}}`},
		{"provider_key_state", ConfigProvider{Name: "p", HasAPIKey: true}, `{"name":"p","base_url":"","has_api_key":true,"protocol":"","source":""}`},
		{"provider_enabled", ProviderEnabledRequest{Enabled: true}, "{\"enabled\":true}"},
		{"account_enabled", AccountEnabledRequest{Enabled: true}, `{"enabled":true}`},
		{"login_start", LoginStartRequest{Service: "codex"}, "{\"service\":\"codex\"}"},
		{"login_status", LoginStatusResponse{Status: "wait"}, "{\"status\":\"wait\"}"},
		{"skill_enabled", SkillEnabledRequest{Enabled: false}, "{\"enabled\":false}"},
		{"helper_status", OAuthHelperResponse{Installed: true, Running: true, Ready: true, Version: "v8.1.0", Port: 1900, Accounts: 2, HasAccounts: true}, `{"installed":true,"running":true,"ready":true,"version":"v8.1.0","port":1900,"accounts":2,"has_accounts":true}`},
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
