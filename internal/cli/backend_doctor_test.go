package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func find(r DoctorReport, name string) (Check, bool) {
	for _, c := range r.Checks {
		if c.Name == name {
			return c, true
		}
	}
	return Check{}, false
}

// Without a daemon doctor still diagnoses the socket and the local files.
func TestDoctorWithoutDaemonStillChecksLocalState(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "settings.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := New(home, "", "", "", "").Doctor(DoctorRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := find(r, "daemon socket"); !ok || c.Status != "error" || c.Fix == "" {
		t.Fatalf("socket check: %+v", c)
	}
	if c, ok := find(r, "settings.json"); !ok || c.Status != "error" {
		t.Fatalf("config check: %+v", c)
	}
}

// A corrupt database is reported as an error with a fix, not repaired.
func TestDoctorReportsCorruptDatabase(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "meno.db")
	if err := os.WriteFile(path, []byte(strings.Repeat("not a database ", 100)), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	c := databaseCheck(home)
	if c.Status != "error" {
		t.Fatalf("check: %+v", c)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatal("doctor must not modify the database")
	}
}

// The output never carries file contents, only a verdict.
func TestDoctorDoesNotLeakSecrets(t *testing.T) {
	home := t.TempDir()
	_ = os.WriteFile(filepath.Join(home, "provider.json"), []byte(`{"api_key":"sk-SECRET"`), 0o600)
	r, _ := New(home, "", "", "", "").Doctor(DoctorRequest{})
	for _, c := range r.Checks {
		if strings.Contains(c.Detail+c.Fix+c.Name, "SECRET") {
			t.Fatalf("leaked: %+v", c)
		}
	}
}

// The daemon reports "v0.6.0" and the client knows "0.6.0": same version.
func TestDoctorTreatsVPrefixedVersionAsEqual(t *testing.T) {
	if strings.TrimPrefix("v0.6.0", "v") != "0.6.0" {
		t.Fatal("prefix handling")
	}
}

func TestDoctorReportsAMissingSystemPromptFile(t *testing.T) {
	home := t.TempDir()
	if got := systemPromptCheck(home); got != nil {
		t.Fatalf("no settings file: %+v", got)
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(home, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("settings.json", `{"system_prompt":"house"}`)
	got := systemPromptCheck(home)
	if len(got) != 1 || got[0].Status != "warn" || !strings.Contains(got[0].Fix, "house.md") {
		t.Fatalf("missing file: %+v", got)
	}
	if err := os.MkdirAll(filepath.Join(home, "system-prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("system-prompts/house.md", "House rules.")
	if got = systemPromptCheck(home); len(got) != 1 || got[0].Status != "ok" {
		t.Fatalf("usable file: %+v", got)
	}
	write("settings.json", `{"system_prompt":"default"}`)
	if got = systemPromptCheck(home); got != nil {
		t.Fatalf("default: %+v", got)
	}
}
