package config

import (
	"path/filepath"
	"testing"

	"meno/internal/llm"
)

func TestSettingsCycleWraps(t *testing.T) {
	s := DefaultSettings()
	if got := s.Cycle("js_code", 1); got != "lines5" {
		t.Fatalf("want lines5 got %s", got)
	}
	s.Cycle("js_code", 1)
	if got := s.Cycle("js_code", 1); got != "full" {
		t.Fatalf("wrap forward failed: %s", got)
	}
	if got := s.Cycle("js_code", -1); got != "off" {
		t.Fatalf("wrap backward failed: %s", got)
	}
}

func TestSettingsSetValidates(t *testing.T) {
	s := DefaultSettings()
	if err := s.Set("js_timeout", "5s"); err != nil || s.JSTimeout != "5s" {
		t.Fatalf("valid set failed: %v", err)
	}
	if err := s.Set("js_timeout", "1h"); err == nil {
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
	if err != nil || s.Model != "gpt-5.5" {
		t.Fatalf("defaults: %v %+v", err, s)
	}
	ps, err := st.LoadProviders()
	if err != nil || len(ps) != 4 {
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
