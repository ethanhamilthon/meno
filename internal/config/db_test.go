package config

import (
	"os"
	"testing"

	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
)

func TestSaveSettingsDropsLegacyJSKeys(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	t.Cleanup(db.CloseAll)
	d, err := st.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`INSERT INTO settings(key,value_json) VALUES('js_code','"lines"')`); err != nil {
		t.Fatal(err)
	}
	s := DefaultSettings()
	if err := st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.QueryRow("SELECT COUNT(*) FROM settings WHERE key LIKE 'js_%'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("legacy keys %d %v", n, err)
	}
}

func TestDBRoundTrips(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	t.Cleanup(db.CloseAll)
	s := DefaultSettings()
	s.Model, s.SystemPrompt = "gpt", "nord"
	if err := st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if got, err := st.LoadSettings(); err != nil || got != s {
		t.Fatalf("%+v %v", got, err)
	}
	sv := []mcp.Server{
		{Name: "a", Transport: mcp.TransportStdio, Command: "x y", Env: map[string]string{"K": "v"}, Enabled: true},
		{Name: "b", Transport: "sse", URL: "http://u", Headers: map[string]string{"A": "b"}},
		{Name: "c", Transport: mcp.TransportStdio, Command: "python3", Args: []string{"server.py", "--flag"}, Cwd: "/tmp/project", Enabled: true},
	}
	if err := st.SaveServers(sv); err != nil {
		t.Fatal(err)
	}
	got, _ := st.LoadServers()
	if len(got) != 3 || got[0].Env["K"] != "v" || got[1].Headers["A"] != "b" || got[1].Enabled || got[0].Name != "a" {
		t.Fatalf("%+v", got)
	}
	if len(got[2].Args) != 2 || got[2].Args[1] != "--flag" || got[2].Cwd != "/tmp/project" {
		t.Fatalf("args and cwd were not kept: %+v", got[2])
	}
	cat := map[string]llm.ModelInfo{"m": {Context: 10, Input: 1.5}}
	st.SaveCatalog(cat)
	if c, _ := st.LoadCatalog(); c["m"].Context != 10 || c["m"].Input != 1.5 {
		t.Fatalf("%+v", c)
	}
	d, _ := st.DB()
	var v int
	d.QueryRow("PRAGMA user_version").Scan(&v)
	if v != db.Version {
		t.Fatal(v)
	}
	if fi, _ := os.Stat(st.P.File(db.File)); fi.Mode().Perm() != 0o600 {
		t.Fatal(fi.Mode())
	}
}
