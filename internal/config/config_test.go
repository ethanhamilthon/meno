package config

import (
	"path/filepath"
	"testing"
	"time"

	"meno/internal/llm"
)

func TestSettingsCycleWraps(t *testing.T) {
	s := DefaultSettings()
	if got := s.Cycle("js_code", 1); got != "lines" {
		t.Fatalf("want lines got %s", got)
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

func TestAdjustLinesClamps(t *testing.T) {
	s := DefaultSettings()
	if s.AdjustLines(-100) != 1 || s.AdjustLines(100) != 50 {
		t.Fatal("clamp failed")
	}
}

func TestTimeoutDuration(t *testing.T) {
	s := DefaultSettings()
	if s.TimeoutDuration() != 30*time.Second {
		t.Fatal("30s")
	}
	s.JSTimeout = "none"
	if s.TimeoutDuration() != 0 {
		t.Fatal("none")
	}
}

func TestServersRoundTrip(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	sv, _ := st.LoadServers()
	if len(sv) != 4 {
		t.Fatalf("defaults %d", len(sv))
	}
	if err := st.SaveServers(sv[:1]); err != nil {
		t.Fatal(err)
	}
	got, _ := st.LoadServers()
	if len(got) != 1 {
		t.Fatalf("roundtrip %d", len(got))
	}
}

func TestPromptValidation(t *testing.T) {
	if ValidatePrompt(Prompt{Name: "Bad Name", Template: "x"}) == nil {
		t.Fatal("bad name accepted")
	}
	if ValidatePrompt(Prompt{Name: "ok-1", Template: "  "}) == nil {
		t.Fatal("empty template accepted")
	}
	if ValidatePrompt(Prompt{Name: "#ok-1", Template: "x"}) != nil {
		t.Fatal("hash prefix should be allowed")
	}
}

func TestFilterSkills(t *testing.T) {
	sk := DefaultSkills()
	if len(FilterSkills(sk, "api")) != 1 {
		t.Fatal("api filter")
	}
	if len(FilterSkills(sk, "")) != len(sk) {
		t.Fatal("empty filter")
	}
}
