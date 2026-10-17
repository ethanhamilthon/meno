package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/config"
	"meno/internal/mcp"
)

// runUI plays the UI loop: it handles events on one goroutine until the test ends.
func runUI(t *testing.T, a *App) {
	t.Helper()
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case e := <-a.events:
				a.handle(e)
			case <-stop:
				return
			}
		}
	}()
	t.Cleanup(func() { close(stop); <-done })
}

func TestAgentManagesMCPServers(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fake server.py")
	if err := os.WriteFile(script, []byte(pyServer), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _ := appWithServers(t, nil)
	runUI(t, a)
	js := func(code string) string {
		t.Helper()
		r := a.vm.Exec(contextBackground(), code)
		if r.Err != "" {
			t.Fatalf("%s: %s", code, r.Err)
		}
		return r.Value
	}
	cmd := "python3 '" + script + "'"
	if got := js("JSON.stringify(mcpServers.list())"); got != "[]" {
		t.Fatalf("list %s", got)
	}
	got := js("var r = mcpServers.add({name: 'fake', command: " + jsString(cmd) + ", env: {TOKEN: 'secret'}}); JSON.stringify(r)")
	if !strings.Contains(got, `"ok":true`) || !strings.Contains(got, "echo") {
		t.Fatalf("add: %s", got)
	}
	if got := js("mcp.fake.echo({msg: 'hi'})"); got != "echo:hi" {
		t.Fatalf("call %q", got)
	}
	list := js("JSON.stringify(mcpServers.list())")
	if strings.Contains(list, "secret") || !strings.Contains(list, `"TOKEN":"***"`) || !strings.Contains(list, `"status":"running"`) {
		t.Fatalf("list %s", list)
	}
	if saved, _ := config.NewStore(a.paths).LoadServers(); len(saved) != 1 || saved[0].Env["TOKEN"] != "secret" || saved[0].Transport != "stdio" {
		t.Fatalf("not persisted: %+v", saved)
	}
	if got := js("mcpServers.add({name: 'fake', command: 'x'}).error"); !strings.Contains(got, "already exists") {
		t.Fatalf("duplicate: %s", got)
	}
	if got := js("mcpServers.add({name: 'bad', commnad: 'x'}).error"); !strings.Contains(got, "unknown field") {
		t.Fatalf("typo: %s", got)
	}
	if got := js("JSON.stringify(mcpServers.add({name: 'broken', command: '/no/such/binary'}))"); !strings.Contains(got, `"ok":false`) || !strings.Contains(got, `"saved":true`) {
		t.Fatalf("broken: %s", got)
	}
	if got := js("mcpTools('fake').length + ' ' + mcpTools('nope').length"); got != "1 0" {
		t.Fatalf("mcpTools filter: %s", got)
	}
	if got := js("mcpServers.update('fake', {enabled: false}).ok + ' ' + mcpServers.list()[0].status"); got != "true stopped" {
		t.Fatalf("disable: %s", got)
	}
	if got := js("mcpServers.restart('fake').error"); !strings.Contains(got, "disabled") {
		t.Fatalf("restart disabled: %s", got)
	}
	if got := js("mcpServers.update('fake', {enabled: true, name: 'renamed'}).ok + ' ' + mcp.renamed.echo({msg: 'x'})"); got != "true echo:x" {
		t.Fatalf("rename: %s", got)
	}
	if got := js("mcpServers.remove('renamed').ok + ' ' + mcpServers.remove('renamed').ok"); got != "true false" {
		t.Fatalf("remove: %s", got)
	}
	if a.mcp.Running("renamed") || a.mcp.Running("fake") {
		t.Fatal("removed server still running")
	}
	if saved, _ := config.NewStore(a.paths).LoadServers(); len(saved) != 1 || saved[0].Name != "broken" {
		t.Fatalf("persisted %+v", saved)
	}
	if n := a.lastNotice(); !strings.Contains(n, "mcp") {
		t.Fatalf("no timeline event: %q", n)
	}
}

func TestAgentRemovingServerClosesItsEditForm(t *testing.T) {
	a, _ := appWithServers(t, []mcp.Server{{Name: "a", Transport: "stdio", Command: "x"}, {Name: "b", Transport: "stdio", Command: "y"}})
	runUI(t, a)
	done := make(chan struct{})
	a.postEvent(uiCall{func() { a.openMCPEdit(1); close(done) }, make(chan struct{})})
	<-done
	// Removing the other server only shifts the index.
	if res := (mcpAdmin{a}).Remove("a"); res["ok"] != true {
		t.Fatal(res)
	}
	a.onUI(func() {
		if a.st.Over != OverMCPEdit || a.st.EditIdx != 0 {
			t.Errorf("over %v idx %d", a.st.Over, a.st.EditIdx)
		}
	})
	if res := (mcpAdmin{a}).Remove("b"); res["ok"] != true {
		t.Fatal(res)
	}
	a.onUI(func() {
		if a.st.Over != OverMCP || !strings.Contains(a.lastNotice(), "removed") {
			t.Errorf("edit form survived removal: over %v notice %q", a.st.Over, a.lastNotice())
		}
	})
}

func jsString(s string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
}
