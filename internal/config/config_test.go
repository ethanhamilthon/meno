package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"meno/internal/llm"
	"meno/internal/mcp"
)

func TestSettingsCycleWraps(t *testing.T) {
	s := DefaultSettings()
	if got := s.Cycle("js_code", 1); got != "lines" {
		t.Fatalf("want lines got %s", got)
	}
	s.Cycle("js_code", 1)
	if got := s.Cycle("js_code", 1); got != "all" {
		t.Fatalf("wrap forward failed: %s", got)
	}
	if got := s.Cycle("js_code", -1); got != "off" {
		t.Fatalf("wrap backward failed: %s", got)
	}
}

func TestSettingsSetValidates(t *testing.T) {
	s := DefaultSettings()
	if err := s.Set("js_timeout", "model"); err != nil || s.JSTimeout != "model" || !s.ModelTimeout() {
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

func TestAdjustNumClamps(t *testing.T) {
	s := DefaultSettings()
	if s.AdjustNum("js_code", -100) != 1 || s.AdjustNum("js_code", 100) != 50 {
		t.Fatal("clamp failed")
	}
	// code, output and thinking keep independent limits
	if s.AdjustNum("js_output", -100) != 1 || s.JSLines != 50 || s.NumFor("js_output") != 1 || s.NumFor("thinking") != 5 {
		t.Fatal("limits are not independent")
	}
	if s.AdjustNum("thinking", 3) != 8 || s.ThinkLines != 8 {
		t.Fatal("thinking limit")
	}
}

func TestAdjustTimeoutSteps(t *testing.T) {
	s := DefaultSettings()
	var up []int
	for i := 0; i < 5; i++ {
		up = append(up, s.AdjustNum("js_timeout", 1))
	}
	if want := []int{35, 40, 45, 50, 55}; !reflect.DeepEqual(up, want) {
		t.Fatalf("up %v want %v", up, want)
	}
	s.JSTimeoutSec = 10
	if s.AdjustNum("js_timeout", -1) != 9 || s.AdjustNum("js_timeout", 1) != 10 {
		t.Fatal("step is not symmetric around 10")
	}
	s.JSTimeoutSec = 60
	if s.AdjustNum("js_timeout", 1) != 90 || s.AdjustNum("js_timeout", -1) != 60 {
		t.Fatal("step is not symmetric around 60")
	}
	s.JSTimeoutSec = 1
	if s.AdjustNum("js_timeout", -1) != 1 {
		t.Fatal("below 1 second")
	}
	s.JSTimeoutSec = MaxTimeoutSec
	if s.AdjustNum("js_timeout", 1) != MaxTimeoutSec {
		t.Fatal("above max")
	}
	if NumOption("js_timeout") != "sec" || NumOption("thinking") != "lines" || NumOption("effort") != "" {
		t.Fatal("NumOption")
	}
}

func TestTimeoutDuration(t *testing.T) {
	s := DefaultSettings()
	if s.TimeoutDuration() != 30*time.Second {
		t.Fatal("30s")
	}
	s.JSTimeoutSec = 7
	if s.TimeoutDuration() != 7*time.Second {
		t.Fatal("7s")
	}
	s.JSTimeout = "off"
	if s.TimeoutDuration() != 0 {
		t.Fatal("off")
	}
	s.JSTimeout = "model"
	if s.TimeoutDuration() != DefaultTimeout {
		t.Fatal("model default")
	}
}

func TestLegacyPlaceholderServersAreDropped(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	old := `[{"name":"postgres","transport":"stdio","command":"npx -y @mcp/postgres","env":{"PG_URL":"x"},"enabled":true},{"name":"github","transport":"stdio","command":"gh-mcp-server","enabled":true},{"name":"sentry","transport":"sse","command":"","url":"http://localhost:9100","enabled":true},{"name":"playwright","transport":"stdio","command":"npx -y @mcp/playwright","enabled":false}]`
	if err := os.WriteFile(st.P.File("mcp.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	sv, err := st.LoadServers()
	if err != nil || len(sv) != 0 {
		t.Fatalf("placeholders survived: %v %v", sv, err)
	}
	if b, _ := os.ReadFile(st.P.File("mcp.json")); strings.Contains(string(b), "postgres") {
		t.Fatal("placeholders still on disk")
	}
	// One real change keeps the whole file.
	mine := strings.Replace(old, "gh-mcp-server", "my-github-server", 1)
	if err := os.WriteFile(st.P.File("mcp.json"), []byte(mine), 0o600); err != nil {
		t.Fatal(err)
	}
	if sv, _ := st.LoadServers(); len(sv) != 4 {
		t.Fatalf("user servers dropped: %d", len(sv))
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
