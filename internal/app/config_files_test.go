package app

import (
	"os"
	"strings"
	"testing"

	"meno/internal/config"
	"meno/internal/mcp"
)

func TestFileProvidersAndServersAreShownButNeverSaved(t *testing.T) {
	a, scr, paths := e2eApp(t, &recClient{})
	defer scr.Fini()
	os.WriteFile(paths.ProviderFile(), []byte(`[{"name":"extra","base_url":"http://e/v1","api_key":"k","protocol":"responses"}]`), 0o600)
	os.WriteFile(paths.MCPFile(), []byte(`[{"name":"srv","command":"true","enabled":false}]`), 0o600)
	if err := a.loadAll(); err != nil {
		t.Fatal(err)
	}
	var fileP, fileS bool
	for _, p := range a.st.Providers {
		fileP = fileP || (p.Name == "extra" && p.FromFile())
	}
	for _, s := range a.st.Servers {
		fileS = fileS || (s.Name == "srv" && s.FromFile())
	}
	if !fileP || !fileS {
		t.Fatalf("overlay entries missing: providers=%v servers=%v", fileP, fileS)
	}
	a.persistProviders()
	a.persistServers()
	st := config.NewStore(paths)
	if ps, _ := st.LoadProviders(); len(ps) != 1 || ps[0].Name != "openai" {
		t.Fatalf("a file provider was saved: %+v", ps)
	}
	if sv, _ := st.LoadServers(); len(sv) != 0 {
		t.Fatalf("a file server was saved: %+v", sv)
	}
	// Reloading must not duplicate them.
	if err := a.loadAll(); err != nil {
		t.Fatal(err)
	}
	if len(a.st.Providers) != 2 || len(a.st.Servers) != 1 {
		t.Fatalf("after reload: %d providers, %d servers", len(a.st.Providers), len(a.st.Servers))
	}
}

func TestBrokenOverlayFilesGiveNoticesNotErrors(t *testing.T) {
	a, scr, paths := e2eApp(t, &recClient{})
	defer scr.Fini()
	os.WriteFile(paths.ProviderFile(), []byte("{broken"), 0o600)
	os.WriteFile(paths.ThemeFile(), []byte("[1]"), 0o600)
	a.loadThemeFile()
	if err := a.loadAll(); err != nil {
		t.Fatal(err)
	}
	a.reportWarnings()
	var text []string
	for _, n := range a.st.Notices {
		text = append(text, n.Text)
	}
	got := strings.Join(text, "\n")
	if !strings.Contains(got, "provider.json") || !strings.Contains(got, "theme.json") {
		t.Fatalf("no notices about the broken files: %q", got)
	}
}

func TestGuardRefusesChangesToFileServers(t *testing.T) {
	f := mcp.Server{Name: "f", Command: "x", Transport: "stdio", Enabled: true, Source: mcp.SourceFile}
	own := mcp.Server{Name: "o", Command: "y", Transport: "stdio"}
	if err := guardFileServers([]mcp.Server{f, own}, []mcp.Server{f}); err != nil {
		t.Fatal("removing an own server is fine:", err)
	}
	changed := f
	changed.Command = "z"
	if err := guardFileServers([]mcp.Server{f}, []mcp.Server{changed}); err == nil {
		t.Fatal("editing a file server must be refused")
	}
	if err := guardFileServers([]mcp.Server{f}, nil); err == nil {
		t.Fatal("removing a file server must be refused")
	}
}
