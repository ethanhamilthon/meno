package config

import (
	"os"
	"strings"
	"testing"

	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
)

// seedTables puts providers and servers into the 0.6.7 tables.
func seedTables(t *testing.T, p Paths, ps []llm.Provider, sv []mcp.Server) {
	t.Helper()
	d, err := db.Open(p.Home)
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range ps {
		if _, err = d.Exec("INSERT INTO providers(pos,name,base_url,api_key,protocol,run,disabled) VALUES(?,?,?,?,?,?,?)",
			i, x.Name, x.BaseURL, x.APIKey, x.Protocol, x.Run, b2i(x.Disabled)); err != nil {
			t.Fatal(err)
		}
	}
	for i, x := range sv {
		if _, err = d.Exec("INSERT INTO mcp_servers(pos,name,transport,command,url,enabled) VALUES(?,?,?,?,?,?)",
			i, x.Name, x.Transport, x.Command, x.URL, b2i(x.Enabled)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMigrationMovesTablesIntoFilesOnce(t *testing.T) {
	st, p := storeIn(t)
	t.Cleanup(db.CloseAll)
	seedTables(t, p,
		[]llm.Provider{{Name: "a", BaseURL: "http://x", APIKey: "key", Protocol: llm.ProtoChat, Disabled: true}},
		[]mcp.Server{{Name: "s", Transport: "stdio", Command: "echo hi", Enabled: true}})
	if err := MigrateToFiles(p); err != nil {
		t.Fatal(err)
	}
	ps, _ := st.LoadProviders()
	sv, _ := st.LoadServers()
	if len(ps) != 1 || ps[0].APIKey != "key" || !ps[0].Disabled || len(sv) != 1 || sv[0].Command != "echo hi" {
		t.Fatalf("%+v %+v", ps, sv)
	}
	if _, err := os.Stat(p.ProviderFile() + backupTag); err == nil {
		t.Fatal("there was no file to back up")
	}
	// A second run changes nothing, even if the user edited the file meanwhile.
	if err := st.SaveProviders(nil); err != nil {
		t.Fatal(err)
	}
	if err := MigrateToFiles(p); err != nil {
		t.Fatal(err)
	}
	if ps, _ = st.LoadProviders(); len(ps) != 0 {
		t.Fatalf("the migration ran twice: %+v", ps)
	}
}

func TestMigrationKeepsFileEntriesAndBacksUpTheFile(t *testing.T) {
	st, p := storeIn(t)
	t.Cleanup(db.CloseAll)
	seedTables(t, p, []llm.Provider{{Name: "a", BaseURL: "http://db", Protocol: llm.ProtoChat}}, nil)
	old := `[{"name":"a","base_url":"http://file","protocol":"responses"},{"name":"f","base_url":"http://f","protocol":"responses"},{"name":"junk","base_url":"ftp://x","protocol":"responses"}]`
	writeFile(t, p.ProviderFile(), old)
	if err := MigrateToFiles(p); err != nil {
		t.Fatal(err)
	}
	ps, _ := st.LoadProviders()
	if len(ps) != 2 || ps[0].Name != "a" || ps[0].BaseURL != "http://db" || ps[1].Name != "f" {
		t.Fatalf("database entry must win, file-only entry must stay: %+v", ps)
	}
	if readText(t, p.ProviderFile()+backupTag) != old {
		t.Fatal("the old file was not backed up")
	}
	if !strings.Contains(readText(t, p.ProviderFile()), "junk") {
		t.Fatal("an unreadable entry was dropped")
	}
}

func TestMigrationFailureIsRetried(t *testing.T) {
	st, p := storeIn(t)
	t.Cleanup(db.CloseAll)
	seedTables(t, p, nil, []mcp.Server{{Name: "s", Transport: "stdio", Command: "echo hi", Enabled: true}})
	writeFile(t, p.MCPFile(), "{broken")
	if err := MigrateToFiles(p); err == nil {
		t.Fatal("a broken file must stop the migration")
	}
	if readText(t, p.MCPFile()) != "{broken" {
		t.Fatal("the broken file was changed")
	}
	writeFile(t, p.MCPFile(), "[]")
	if err := MigrateToFiles(p); err != nil {
		t.Fatal(err)
	}
	if sv, _ := st.LoadServers(); len(sv) != 1 {
		t.Fatalf("not retried: %+v", sv)
	}
}

func TestMigrationWithoutDatabaseDoesNothing(t *testing.T) {
	_, p := storeIn(t)
	if err := MigrateToFiles(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ProviderFile()); err == nil {
		t.Fatal("a file was created")
	}
	if _, err := os.Stat(p.File(db.File)); err == nil {
		t.Fatal("a database was created")
	}
}
