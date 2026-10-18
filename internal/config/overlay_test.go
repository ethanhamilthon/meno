package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/llm"
	"meno/internal/mcp"
)

func writeOverlay(t *testing.T, name, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProviderFileLoadsValidEntriesAndWarnsAboutTheRest(t *testing.T) {
	p := writeOverlay(t, "provider.json", `[
		{"name":"a","base_url":"http://x/v1","api_key":"env:A_KEY","protocol":"responses"},
		{"name":"bad","base_url":"ftp://x","protocol":"responses"},
		{"name":"a","base_url":"http://y","protocol":"responses"},
		{"name":"b","base_url":"https://z","api_key":"env:MISSING","protocol":"chat completions"}
	]`)
	env := map[string]string{"A_KEY": "secret"}
	got, warns := LoadProviderFile(p, func(k string) string { return env[k] })
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("providers: %+v", got)
	}
	if got[0].APIKey != "secret" || !got[0].FromFile() {
		t.Fatalf("env key or source not applied: %+v", got[0])
	}
	text := strings.Join(warns, "\n")
	for _, want := range []string{"bad", "duplicate name", "MISSING"} {
		if !strings.Contains(text, want) {
			t.Fatalf("no warning about %q in:\n%s", want, text)
		}
	}
}

func TestOverlayFilesMissingEmptyOrBrokenNeverFail(t *testing.T) {
	if got, w := LoadProviderFile(filepath.Join(t.TempDir(), "provider.json"), os.Getenv); got != nil || w != nil {
		t.Fatalf("missing file: %v %v", got, w)
	}
	if got, w := LoadMCPFile(writeOverlay(t, "mcp.json", "  \n")); got != nil || w != nil {
		t.Fatalf("empty file: %v %v", got, w)
	}
	if got, w := LoadMCPFile(writeOverlay(t, "mcp.json", "{not json")); got != nil || len(w) != 1 {
		t.Fatalf("broken file: %v %v", got, w)
	}
	if got, w := LoadProviderFile(writeOverlay(t, "provider.json", `{"name":"x"}`), os.Getenv); got != nil || len(w) != 1 {
		t.Fatalf("object instead of array: %v %v", got, w)
	}
	if got, w := LoadProviderFile(writeOverlay(t, "provider.json", `[1,"x",null]`), os.Getenv); len(got) != 0 || len(w) == 0 {
		t.Fatalf("junk entries: %v %v", got, w)
	}
}

func TestMCPFileDefaultsToEnabled(t *testing.T) {
	p := writeOverlay(t, "mcp.json", `[
		{"name":"on","command":"echo hi"},
		{"name":"off","command":"echo hi","enabled":false},
		{"name":"","command":"x"}
	]`)
	got, warns := LoadMCPFile(p)
	if len(got) != 2 || !got[0].Enabled || got[1].Enabled || got[0].Transport != "stdio" || !got[0].FromFile() {
		t.Fatalf("servers: %+v", got)
	}
	if len(warns) != 1 {
		t.Fatalf("warnings: %v", warns)
	}
}

func TestMergeKeepsSavedEntriesAndNeverDuplicates(t *testing.T) {
	saved := []llm.Provider{{Name: "a", BaseURL: "http://x", Protocol: llm.ProtoChat}}
	file := []llm.Provider{
		{Name: "a", BaseURL: "http://x", Protocol: llm.ProtoChat, Source: llm.SourceFile}, // identical: silent
		{Name: "b", BaseURL: "http://y", Protocol: llm.ProtoChat, Source: llm.SourceFile},
	}
	got, warns := MergeProviders(saved, file)
	if len(got) != 2 || got[1].Name != "b" || len(warns) != 0 {
		t.Fatalf("%+v %v", got, warns)
	}
	file[0].BaseURL = "http://other"
	got, warns = MergeProviders(saved, file)
	if len(got) != 2 || got[0].BaseURL != "http://x" || len(warns) != 1 {
		t.Fatalf("saved entry must win: %+v %v", got, warns)
	}

	sv, w := MergeServers([]mcp.Server{{Name: "s", Command: "a", Transport: "stdio"}},
		[]mcp.Server{{Name: "s", Command: "b", Transport: "stdio", Source: mcp.SourceFile}})
	if len(sv) != 1 || sv[0].Command != "a" || len(w) != 1 {
		t.Fatalf("%+v %v", sv, w)
	}
}

func TestOwnEntriesDropFileOnes(t *testing.T) {
	ps := []llm.Provider{{Name: "a"}, {Name: "f", Source: llm.SourceFile}}
	if got := OwnProviders(ps); len(got) != 1 || got[0].Name != "a" {
		t.Fatal(got)
	}
	sv := []mcp.Server{{Name: "a"}, {Name: "f", Source: mcp.SourceFile}}
	if got := OwnServers(sv); len(got) != 1 || got[0].Name != "a" {
		t.Fatal(got)
	}
}
