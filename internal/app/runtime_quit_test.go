package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
)

func TestQuitAndForceQuit(t *testing.T) {
	cl := newGateClient()
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	start(a, "alpha")
	a.runCommand("new")
	a.runCommand("q")
	if a.quit || !strings.Contains(a.lastNotice(), "alpha") || !strings.Contains(a.lastNotice(), "running") {
		t.Fatalf("q must refuse and name the session: %q", a.lastNotice())
	}
	a.runCommand("q!")
	if !a.quit {
		t.Fatal("q! quits")
	}
	pump(t, a, func() bool { return !a.rtList[0].Running })
	// A session that is idle does not hold q back.
	b, scr2, _ := e2eApp(t, newGateClient())
	defer scr2.Fini()
	b.runCommand("q")
	if !b.quit {
		t.Fatal("idle q")
	}
}

func TestCdChangesTheSessionDirectory(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	sub := filepath.Join(paths.Cwd, "sub")
	os.MkdirAll(filepath.Join(sub, ".meno", "skills"), 0o755)
	os.WriteFile(filepath.Join(sub, ".meno", "skills", "local.md"), []byte("---\ndescription: local\n---\n"), 0o644)
	os.WriteFile(filepath.Join(sub, "marker.txt"), []byte("in sub"), 0o644)
	os.WriteFile(filepath.Join(paths.Cwd, "marker.txt"), []byte("in root"), 0o644)
	other := filepath.Join(a.paths.Home, "..")
	_ = other

	// Validation.
	a.runCommand("cd nonexistent")
	if !strings.Contains(a.lastNotice(), "no such directory") || a.st.Cwd != paths.Cwd {
		t.Fatalf("missing dir: %q", a.lastNotice())
	}
	a.runCommand("cd marker.txt")
	if !strings.Contains(a.lastNotice(), "not a directory") {
		t.Fatalf("file: %q", a.lastNotice())
	}
	a.runCommand("cd -")
	if !strings.Contains(a.lastNotice(), "no previous directory") {
		t.Fatalf("dash: %q", a.lastNotice())
	}

	// Empty history: nothing else, the prompt is built for the new directory.
	a.runCommand("cd sub")
	if a.st.Cwd != sub || a.st.VM.Root() != sub || a.st.Session.Cwd != sub {
		t.Fatalf("cwd %q root %q session %q", a.st.Cwd, a.st.VM.Root(), a.st.Session.Cwd)
	}
	say(t, a, "first")
	r := cl.requests()[0]
	if lastUser(r) != "first" || !strings.Contains(r.System, "Working directory: "+sub) || !strings.Contains(r.System, "- local") {
		t.Fatalf("first request: user %q\n%s", lastUser(r), r.System)
	}

	// Relative paths: fs and sh follow.
	if res := a.st.VM.Exec(context.Background(), "fs.read('marker.txt')"); res.Value != "in sub" {
		t.Fatalf("%+v", res)
	}
	a.runCommand("cd ..")
	if a.st.Cwd != paths.Cwd || a.st.PrevCwd != sub {
		t.Fatalf("cd ..: %q prev %q", a.st.Cwd, a.st.PrevCwd)
	}
	if res := a.st.VM.Exec(context.Background(), "fs.read('marker.txt')"); res.Value != "in root" {
		t.Fatalf("%+v", res)
	}
	// cd - goes back; non-empty history: the prompt stays, events tell the model.
	a.runCommand("cd -")
	a.runCommand("cd ~")
	home, _ := os.UserHomeDir()
	if a.st.Cwd != home {
		t.Fatalf("~: %q", a.st.Cwd)
	}
	a.runCommand("cd " + paths.Cwd)
	say(t, a, "second")
	r2 := cl.requests()[1]
	got := lastUser(r2)
	if r2.System != r.System {
		t.Fatal("cd must not rebuild the prompt")
	}
	if !strings.Contains(got, "<cwd-changed>The working directory changed from "+sub+" to "+paths.Cwd) || !strings.Contains(got, "<skills-removed>Skill \"local\"") {
		t.Fatalf("event: %q", got)
	}
	// The directory is saved with the session.
	saved, err := agent.LoadSession(a.paths.Home, a.st.Session.ID)
	if err != nil || saved.Cwd != paths.Cwd {
		t.Fatalf("saved cwd %q err %v", saved.Cwd, err)
	}
}

func TestCdEventOnlyWithHistory(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "one")
	sub := filepath.Join(paths.Cwd, "sub")
	os.MkdirAll(filepath.Join(sub, ".meno", "skills"), 0o755)
	os.WriteFile(filepath.Join(sub, ".meno", "skills", "local.md"), []byte("---\ndescription: local\n---\n"), 0o644)
	os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("sub rules"), 0o644)
	a.runCommand("cd sub")
	say(t, a, "two")
	r := cl.requests()[1]
	got := lastUser(r)
	for _, want := range []string{
		"<cwd-changed>The working directory changed from " + paths.Cwd + " to " + sub,
		"<skills-added>Skill \"local\"",
		"<agents-md-updated>Project instructions were added: read " + filepath.Join(sub, "AGENTS.md"),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(r.System, "sub rules") || !strings.Contains(r.System, "Working directory: "+paths.Cwd) {
		t.Fatal("the frozen prompt must not change")
	}
}

func TestCdRefusedWhileRunning(t *testing.T) {
	cl := newGateClient()
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	os.Mkdir(filepath.Join(paths.Cwd, "sub"), 0o755)
	start(a, "alpha")
	a.runCommand("cd sub")
	if a.st.Cwd != paths.Cwd || !strings.Contains(a.lastNotice(), ":stop") {
		t.Fatalf("cd while running: %q", a.lastNotice())
	}
	cl.open("alpha")
	pump(t, a, func() bool { return !a.st.Running })
}

func TestNewSessionStartsInTheCurrentDirectory(t *testing.T) {
	a, scr, paths := e2eApp(t, &recClient{})
	defer scr.Fini()
	sub := filepath.Join(paths.Cwd, "sub")
	os.Mkdir(sub, 0o755)
	a.runCommand("cd sub")
	a.runCommand("new")
	if a.st.Cwd != sub || a.st.VM.Root() != sub || a.st.Session.Cwd != sub {
		t.Fatalf("new session cwd %q", a.st.Cwd)
	}
	// Each session has its own directory.
	other := a.rts[a.rtList[0].ID()]
	if other != nil && other.ID() != a.st.Session.ID && other.VM == a.st.VM {
		t.Fatal("sessions share a VM")
	}
}

func TestCdCompletion(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"alpha", "alps", "beta", ".hidden"} {
		os.Mkdir(filepath.Join(dir, d), 0o755)
	}
	os.WriteFile(filepath.Join(dir, "alfile"), nil, 0o644)
	got := completeDirs(dir, "al")
	if len(got) != 2 || got[0] != ":cd alpha/" || got[1] != ":cd alps/" {
		t.Fatalf("%v", got)
	}
	if got := completeDirs(dir, ""); len(got) != 3 {
		t.Fatalf("hidden directories need a dot: %v", got)
	}
	if got := completeDirs("/", dir+"/b"); len(got) != 1 || got[0] != ":cd "+dir+"/beta/" {
		t.Fatalf("absolute: %v", got)
	}
	if got := completeDirs(filepath.Dir(dir), "./"+filepath.Base(dir)+"/be"); len(got) != 1 {
		t.Fatalf("relative: %v", got)
	}
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Cwd = dir
	a.st.Cmd = []rune("cd al")
	rows, ok := a.cdRows()
	if !ok || len(rows) != 2 {
		t.Fatalf("%v %v", rows, ok)
	}
	a.st.Mode = 3 // command mode
	a.onCommandKey(keyK(tcell.KeyTab))
	if string(a.st.Cmd) != "cd alpha/" {
		t.Fatalf("tab: %q", string(a.st.Cmd))
	}
}

func TestOpeningStoredSessionWithMissingDirectory(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	gone := filepath.Join(paths.Cwd, "gone")
	os.Mkdir(gone, 0o755)
	a.runCommand("cd gone")
	say(t, a, "one")
	id := a.st.Session.ID
	a.runCommand("new")
	a.dropRuntime(a.rts[id])
	os.Remove(gone)

	a.openSession(id)
	if a.st.Session.ID != id || a.st.Cwd != paths.Cwd || a.st.VM.Root() != paths.Cwd {
		t.Fatalf("must open in the launch directory, got %q", a.st.Cwd)
	}
	if n := a.lastNotice(); !strings.Contains(n, "no longer exists") || !strings.Contains(n, gone) {
		t.Fatalf("notice %q", n)
	}
	say(t, a, "again")
	r := cl.requests()[1]
	got := lastUser(r)
	if !strings.Contains(got, "<cwd-changed>The working directory changed from "+gone+" to "+paths.Cwd) {
		t.Fatalf("events: %q", got)
	}
	if !strings.Contains(r.System, "Working directory: "+paths.Cwd) {
		t.Fatal("prompt must be built for the launch directory")
	}
}
