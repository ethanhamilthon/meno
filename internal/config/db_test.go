package config

import (
	"os"
	"testing"

	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
)

func TestDBRoundTrips(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	t.Cleanup(db.CloseAll)
	s := DefaultSettings()
	s.Model, s.Theme = "gpt", "nord"
	if err := st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if got, err := st.LoadSettings(); err != nil || got != s {
		t.Fatalf("%+v %v", got, err)
	}
	sv := []mcp.Server{{Name: "a", Transport: mcp.TransportStdio, Command: "x y", Env: map[string]string{"K": "v"}, Enabled: true}, {Name: "b", Transport: "sse", URL: "http://u", Headers: map[string]string{"A": "b"}}}
	if err := st.SaveServers(sv); err != nil {
		t.Fatal(err)
	}
	got, _ := st.LoadServers()
	if len(got) != 2 || got[0].Env["K"] != "v" || got[1].Headers["A"] != "b" || got[1].Enabled || got[0].Name != "a" {
		t.Fatalf("%+v", got)
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
