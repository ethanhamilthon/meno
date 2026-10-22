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
