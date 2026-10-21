package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/llm"
)

func TestExportDestination(t *testing.T) {
	cwd := t.TempDir()
	for _, tc := range []struct {
		args        []string
		path, shown string
	}{
		{nil, filepath.Join(cwd, "meno-session.md"), "./meno-session.md"},
		{[]string{"sub/file.md"}, filepath.Join(cwd, "sub/file.md"), "./sub/file.md"},
		{[]string{filepath.Join(cwd, "absolute.md")}, filepath.Join(cwd, "absolute.md"), filepath.Join(cwd, "absolute.md")},
	} {
		path, shown := exportDestination(cwd, tc.args)
		if path != tc.path || shown != tc.shown {
			t.Fatalf("destination %q %q", path, shown)
		}
	}
}

func TestExportAsyncCapturesSessionAndDestination(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "original answer"}, {Kind: llm.EvDone, UsageReported: true}}}})
	defer scr.Fini()
	say(t, a, "original question")
	cwd := t.TempDir()
	a.st.Cwd = cwd
	originalID := a.st.Session.ID()
	a.exportSession(nil)
	// Switching immediately must not affect the captured source or path.
	a.newSession()
	a.st.Cwd = t.TempDir()
	path := filepath.Join(cwd, "meno-session.md")
	pump(t, a, func() bool {
		for _, n := range a.currentNotices() {
			if strings.Contains(n.Text, "session saved to") {
				return true
			}
		}
		return false
	})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{originalID, "original question", "original answer", "## Model requests", "System prompt:", "Function definition:"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %q in export", want)
		}
	}
}

func TestExportReportsWriteFailure(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.exportSession([]string{filepath.Join(t.TempDir(), "missing", "file.md")})
	pump(t, a, func() bool {
		for _, n := range a.currentNotices() {
			if strings.Contains(n.Text, "export failed:") {
				return true
			}
		}
		return false
	})
}

func TestExportReportsRPCFailure(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Session = &sessView{id: "missing-export-session"}
	path := filepath.Join(t.TempDir(), "file.md")
	a.exportSession([]string{path})
	pump(t, a, func() bool {
		for _, n := range a.currentNotices() {
			if strings.Contains(n.Text, "export failed:") {
				return true
			}
		}
		return false
	})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("RPC failure wrote a file: %v", err)
	}
}
