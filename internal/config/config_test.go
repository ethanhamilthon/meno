package config

import (
	"path/filepath"
	"reflect"
	"testing"

	"meno/internal/llm"
	"meno/internal/mcp"
)

func TestSettingsCycleWraps(t *testing.T) {
	s := DefaultSettings()
	if got := s.Cycle("bash_code", 1); got != "lines" {
		t.Fatalf("want lines got %s", got)
	}
	s.Cycle("bash_code", 1)
	if got := s.Cycle("bash_code", 1); got != "all" {
		t.Fatalf("wrap forward failed: %s", got)
	}
	if got := s.Cycle("bash_code", -1); got != "off" {
		t.Fatalf("wrap backward failed: %s", got)
	}
}

func TestSettingsSetValidates(t *testing.T) {
	s := DefaultSettings()
	if err := s.Set("bash_timeout", "1h"); err == nil {
		t.Fatal("an obsolete key must be rejected")
	}
	if err := s.Set("bash_code", "typo"); err == nil {
		t.Fatal("invalid value accepted")
	}
	if err := s.Set("nope", "x"); err == nil {
		t.Fatal("unknown key accepted")
	}
}

func TestSettingsEffortRoundTrip(t *testing.T) {
	s := DefaultSettings()
	if err := s.Set("effort", "max"); err != nil || s.Get("effort") != "max" {
		t.Fatalf("effort: %v", err)
	}
}

func TestStoreSettingsPersist(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	s := DefaultSettings()
	s.Model = "o4-mini"
	if err := st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := st.LoadSettings()
	if err != nil || got.Model != "o4-mini" {
		t.Fatalf("load: %v %+v", err, got)
	}
}

func TestStoreMissingFilesGiveDefaults(t *testing.T) {
	st := NewStore(Paths{Home: filepath.Join(t.TempDir(), "none")})
	s, err := st.LoadSettings()
	if err != nil || s.Model != "" || s.Provider != "" {
		t.Fatalf("no model/provider must be preselected: %v %+v", err, s)
	}
	ps, err := st.LoadProviders()
	if err != nil || len(ps) != 0 {
		t.Fatalf("providers defaults: %v %d", err, len(ps))
	}
}

func TestProvidersRoundTrip(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	in := []llm.Provider{{Name: "x", BaseURL: "https://x", Protocol: llm.ProtoChat}}
	if err := st.SaveProviders(in); err != nil {
		t.Fatal(err)
	}
	out, _ := st.LoadProviders()
	if len(out) != 1 || out[0].Name != "x" {
		t.Fatalf("roundtrip: %+v", out)
	}
}

func TestPathsEnvOverride(t *testing.T) {
	t.Setenv("MENO_HOME", "/tmp/mh")
	p, err := DefaultPaths()
	if err != nil || p.Home != "/tmp/mh" {
		t.Fatalf("home: %v %s", err, p.Home)
	}
	if p.File("a.json") != "/tmp/mh/a.json" {
		t.Fatal("file join")
	}
}

func TestAdjustNumClamps(t *testing.T) {
	s := DefaultSettings()
	if s.AdjustNum("bash_code", -100) != 1 || s.AdjustNum("bash_code", 100) != 50 {
		t.Fatal("clamp failed")
	}
	// code, output and thinking keep independent limits
	if s.AdjustNum("bash_output", -100) != 1 || s.BashLines != 50 || s.NumFor("bash_output") != 1 || s.NumFor("thinking") != 5 {
		t.Fatal("limits are not independent")
	}
	if s.AdjustNum("thinking", 3) != 8 || s.ThinkLines != 8 {
		t.Fatal("thinking limit")
	}
}

func TestServersStartEmptyAndRoundTrip(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	sv, err := st.LoadServers()
	if err != nil || len(sv) != 0 {
		t.Fatalf("a fresh install must have no servers: %v %v", sv, err)
	}
	in := []mcp.Server{
		{Name: "a", Transport: mcp.TransportStdio, Command: "x", Enabled: true},
		{Name: "b", Transport: mcp.TransportHTTP, URL: "https://h/mcp", Headers: map[string]string{"Authorization": "Bearer t"}},
	}
	if err := st.SaveServers(in); err != nil {
		t.Fatal(err)
	}
	got, _ := st.LoadServers()
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("roundtrip %+v", got)
	}
}
