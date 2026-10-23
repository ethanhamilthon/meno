package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePrompt(t *testing.T, p Paths, file, text string) {
	t.Helper()
	if err := os.MkdirAll(p.SystemPromptsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.SystemPromptsDir(), file), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSystemPromptNames(t *testing.T) {
	for _, ok := range []string{"writer", "a.b-c_1", "X"} {
		if err := ValidSystemPromptName(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".hidden", "../x", "a/b", "a b", "ü", strings.Repeat("a", 65)} {
		if ValidSystemPromptName(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestLoadAndListSystemPrompts(t *testing.T) {
	p := Paths{Home: t.TempDir()}
	if got := p.ListSystemPrompts(); len(got) != 0 {
		t.Fatalf("no directory yet: %v", got)
	}
	writePrompt(t, p, "writer.md", "\n You write.\n\n")
	writePrompt(t, p, "blank.md", "  \n")
	writePrompt(t, p, "notes.txt", "x")
	writePrompt(t, p, ".hidden.md", "x")
	writePrompt(t, p, "big.md", strings.Repeat("a", MaxSystemPromptBytes+1))
	if got := p.ListSystemPrompts(); strings.Join(got, ",") != "big,blank,writer" {
		t.Fatalf("list: %v", got)
	}
	if text, err := p.LoadSystemPrompt("writer"); err != nil || text != "You write." {
		t.Fatalf("load: %q %v", text, err)
	}
	if _, err := p.LoadSystemPrompt("nope"); !errors.Is(err, ErrSystemPromptMissing) {
		t.Fatalf("missing: %v", err)
	}
	for _, name := range []string{"blank", "big", "../writer"} {
		if _, err := p.LoadSystemPrompt(name); err == nil {
			t.Errorf("%q loaded", name)
		}
	}
}

func TestSystemPromptSetting(t *testing.T) {
	var s Settings
	if err := json.Unmarshal([]byte(`{"system_prompt":"writer"}`), &s); err != nil || s.SystemPrompt != "writer" || s.Get("system_prompt") != "writer" {
		t.Fatalf("%+v %v", s, err)
	}
	if err := s.Set("system_prompt", DefaultSystemPrompt); err != nil || s.SystemPrompt != "" || s.Get("system_prompt") != DefaultSystemPrompt {
		t.Fatalf("default: %+v %v", s, err)
	}
	if err := s.Set("system_prompt", "../x"); err == nil {
		t.Fatal("a path was accepted")
	}
	// A bad stored name falls back to the built-in prompt; the depth and worker
	// limits get their defaults.
	if err := json.Unmarshal([]byte(`{"system_prompt":"a/b","max_spawn_depth":99,"max_workers":0}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.SystemPrompt != "" || s.MaxSpawnDepth != 2 || s.MaxWorkers != 8 {
		t.Fatalf("%+v", s)
	}
}
