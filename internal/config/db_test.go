package config

import (
	"os"
	"testing"

	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
)

func TestImportLegacyJSONOnceKeepsFiles(t *testing.T) {
	home := t.TempDir()
	st := NewStore(Paths{Home: home})
	os.WriteFile(st.P.File("providers.json"), []byte(`[{"name":"p","base_url":"http://x","api_key":"k","protocol":"chat"}]`), 0o600)
	os.WriteFile(st.P.File("settings.json"), []byte(`{"model":"m1","theme":"nord"}`), 0o600)
	os.WriteFile(st.P.File("skills.json"), []byte(`{"disabled":["off-skill"]}`), 0o600)
	ps, err := st.LoadProviders()
	if err != nil || len(ps) != 1 || ps[0].APIKey != "k" {
		t.Fatalf("%v %v", ps, err)
	}
	if s, _ := st.LoadSettings(); s.Model != "m1" || s.Theme != "nord" {
		t.Fatalf("settings %+v", s)
	}
	// Later edits of the old file are ignored: the database is the source now.
	os.WriteFile(st.P.File("providers.json"), []byte(`[]`), 0o600)
	db.CloseAll()
	st = NewStore(Paths{Home: home})
	if ps, _ := st.LoadProviders(); len(ps) != 1 {
		t.Fatal("legacy file imported twice")
	}
	if _, err := os.Stat(st.P.File("providers.json")); err != nil {
		t.Fatal("legacy file must stay:", err)
	}
	d, _ := st.DB()
	var off int
	d.QueryRow("SELECT COUNT(1) FROM skills_state WHERE name='off-skill' AND enabled=0").Scan(&off)
	if off != 1 {
		t.Fatal("skill state not imported")
	}
}

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
