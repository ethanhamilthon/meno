package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/llm"
	"meno/internal/mcp"
)

func storeIn(t *testing.T) (*Store, Paths) {
	t.Helper()
	p := Paths{Home: t.TempDir()}
	return NewStore(p), p
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestProvidersLoadValidEntriesAndWarnAboutTheRest(t *testing.T) {
	st, p := storeIn(t)
	writeFile(t, p.ProviderFile(), `[
		{"name":"a","base_url":"http://x/v1","api_key":"env:A_KEY","protocol":"responses"},
		{"name":"bad","base_url":"ftp://x","protocol":"responses"},
		{"name":"a","base_url":"http://y","protocol":"responses"},
		{"name":"b","base_url":"https://z","api_key":"env:MISSING","protocol":"chat completions"}
	]`)
	got, warns, err := st.LoadProvidersFile()
	if err != nil || len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("providers: %+v %v", got, err)
	}
	if got[0].APIKey != "env:A_KEY" {
		t.Fatalf("env reference must stay as written: %q", got[0].APIKey)
	}
	text := strings.Join(warns, "\n")
	for _, want := range []string{"bad", "duplicate name"} {
		if !strings.Contains(text, want) {
			t.Fatalf("no warning about %q in:\n%s", want, text)
		}
	}
	env := map[string]string{"A_KEY": "secret"}
	res := ResolveKeys(got, func(k string) string { return env[k] })
	if res[0].APIKey != "secret" || res[1].APIKey != "" || got[0].APIKey != "env:A_KEY" {
		t.Fatalf("resolved %+v, original %+v", res, got)
	}
}

func TestMissingEmptyOrBrokenFilesAreHandled(t *testing.T) {
	st, p := storeIn(t)
	if got, err := st.LoadProviders(); got != nil || err != nil {
		t.Fatalf("missing file: %v %v", got, err)
	}
	writeFile(t, p.MCPFile(), "  \n")
	if got, err := st.LoadServers(); got != nil || err != nil {
		t.Fatalf("empty file: %v %v", got, err)
	}
	writeFile(t, p.MCPFile(), "{not json")
	if _, err := st.LoadServers(); err == nil || !strings.Contains(err.Error(), "mcp.json") {
		t.Fatalf("broken file must name itself: %v", err)
	}
	writeFile(t, p.ProviderFile(), `{"name":"x"}`)
	if _, err := st.LoadProviders(); err == nil {
		t.Fatal("an object instead of an array must be an error")
	}
	writeFile(t, p.ProviderFile(), `[1,"x",null]`)
	if got, warns, err := st.LoadProvidersFile(); err != nil || len(got) != 0 || len(warns) == 0 {
		t.Fatalf("junk entries: %v %v %v", got, warns, err)
	}
}

func TestServersDefaultToEnabledStdio(t *testing.T) {
	st, p := storeIn(t)
	writeFile(t, p.MCPFile(), `[{"name":"a","command":"echo hi"},{"name":"b","command":"echo hi","enabled":false}]`)
	got, err := st.LoadServers()
	if err != nil || len(got) != 2 || !got[0].Enabled || got[1].Enabled || got[0].Transport != mcp.TransportStdio {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	st, p := storeIn(t)
	in := []llm.Provider{
		{Name: "a", BaseURL: "http://x", APIKey: "env:K", Protocol: llm.ProtoChat, Run: "proxy", Disabled: true,
			Models: map[string]llm.ModelSettings{"m": {Context: 1000}}},
		{Name: "b", BaseURL: "https://y", APIKey: "k", Protocol: llm.ProtoResponses},
	}
	if err := st.SaveProviders(in); err != nil {
		t.Fatal(err)
	}
	got, err := st.LoadProviders()
	if err != nil || len(got) != 2 || got[0].Name != "a" || !got[0].Disabled || got[0].Run != "proxy" ||
		got[0].APIKey != "env:K" || got[0].Models["m"].Context != 1000 || got[1].APIKey != "k" {
		t.Fatalf("%+v %v", got, err)
	}
	if fi, _ := os.Stat(p.ProviderFile()); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	sv := []mcp.Server{{Name: "s", Transport: "stdio", Command: "echo", Args: []string{"x"}, Env: map[string]string{"A": "b"}, Enabled: false}}
	if err = st.SaveServers(sv); err != nil {
		t.Fatal(err)
	}
	back, err := st.LoadServers()
	if err != nil || len(back) != 1 || back[0].Enabled || back[0].Args[0] != "x" || back[0].Env["A"] != "b" {
		t.Fatalf("%+v %v", back, err)
	}
}

func TestSaveKeepsEntriesItCouldNotRead(t *testing.T) {
	st, p := storeIn(t)
	writeFile(t, p.ProviderFile(), `[
		{"name":"a","base_url":"http://x","protocol":"responses"},
		{"name":"hand-made","base_url":"ftp://nope","protocol":"responses","note":"mine"}
	]`)
	ps, err := st.LoadProviders()
	if err != nil || len(ps) != 1 {
		t.Fatalf("%+v %v", ps, err)
	}
	ps[0].Disabled = true
	if err = st.SaveProviders(ps); err != nil {
		t.Fatal(err)
	}
	text := readText(t, p.ProviderFile())
	if !strings.Contains(text, "hand-made") || !strings.Contains(text, `"note": "mine"`) || !strings.Contains(text, `"disabled": true`) {
		t.Fatalf("file after save:\n%s", text)
	}
}

func TestSaveNeverOverwritesABrokenFile(t *testing.T) {
	st, p := storeIn(t)
	writeFile(t, p.MCPFile(), "{not json")
	err := st.SaveServers([]mcp.Server{{Name: "x", Transport: "stdio", Command: "echo", Enabled: true}})
	if err == nil || !strings.Contains(err.Error(), "does not overwrite") {
		t.Fatalf("error: %v", err)
	}
	if readText(t, p.MCPFile()) != "{not json" {
		t.Fatal("the broken file was changed")
	}
}

func TestSavedFileIsNeverLeftHalfWritten(t *testing.T) {
	st, p := storeIn(t)
	if err := st.SaveProviders(nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(readText(t, p.ProviderFile())); got != "[]" {
		t.Fatalf("empty list is %q", got)
	}
	left, _ := filepath.Glob(filepath.Join(p.Home, ".*.tmp"))
	if len(left) != 0 {
		t.Fatalf("temporary files: %v", left)
	}
}
