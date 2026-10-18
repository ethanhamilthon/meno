package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/mcp"
)

const pyServer = `import sys, json
for line in sys.stdin:
    m = json.loads(line)
    if "id" not in m:
        continue
    meth = m.get("method")
    if meth == "initialize":
        r = {"protocolVersion": "2025-06-18", "capabilities": {}, "serverInfo": {"name": "fake", "version": "1"}}
    elif meth == "tools/list":
        r = {"tools": [{"name": "echo", "description": "echo back", "inputSchema": {"type": "object", "properties": {"msg": {"type": "string"}}, "required": ["msg"]}}]}
    elif meth == "tools/call":
        r = {"content": [{"type": "text", "text": "echo:" + str(m["params"]["arguments"].get("msg"))}]}
    else:
        r = {}
    sys.stdout.write(json.dumps({"jsonrpc": "2.0", "id": m["id"], "result": r}) + "\n")
    sys.stdout.flush()
`

func appWithServers(t *testing.T, servers []mcp.Server) (*App, tcell.SimulationScreen) {
	t.Helper()
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	scr.SetSize(96, 30)
	paths := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
	if err := config.NewStore(paths).SaveServers(servers); err != nil {
		t.Fatal(err)
	}
	a, err := New(scr, paths, func(llm.Provider) llm.Client { return nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close(); scr.Fini() })
	return a, scr
}

// waitStarted feeds start results into the UI until n servers have reported.
func waitStarted(t *testing.T, a *App, n int) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for seen := 0; seen < n; {
		select {
		case e := <-a.events:
			if _, ok := e.(mcpStarted); ok {
				seen++
			}
			a.handle(e)
		case <-deadline:
			t.Fatal("servers did not finish starting")
		}
	}
}

func TestSystemPromptListsEnabledSkillsOnly(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	if got := a.systemPrompt(); strings.Contains(got, "## Project instructions") || strings.Contains(got, "## Skills") {
		t.Fatalf("a bare install must send the bare prompt:\n%s", got)
	}
	dir := a.paths.ProjectSkillsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"on", "off"} {
		if err := os.WriteFile(filepath.Join(dir, n+".md"), []byte("---\ndescription: the "+n+" skill\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a.reloadSkills()
	for i := range a.st.Skills {
		a.st.Skills[i].Enabled = a.st.Skills[i].Name == "on"
	}
	if err := a.store.SaveSkills(a.st.Skills); err != nil {
		t.Fatal(err)
	}
	got := a.systemPrompt()
	if !strings.Contains(got, "- on — the on skill ("+filepath.Join(dir, "on.md")+")") || strings.Contains(got, "the off skill") {
		t.Fatalf("prompt:\n%s", got)
	}
}

func TestMCPServersStartAtLaunchAndReachJS(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fake server.py")
	if err := os.WriteFile(script, []byte(pyServer), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _ := appWithServers(t, []mcp.Server{
		{Name: "fake", Transport: mcp.TransportStdio, Command: "python3 '" + script + "'", Enabled: true},
		{Name: "off", Transport: mcp.TransportStdio, Command: "python3 '" + script + "'", Enabled: false},
		{Name: "broken", Transport: mcp.TransportStdio, Command: "/no/such/binary", Enabled: true},
	})
	a.startServers()
	waitStarted(t, a, 2) // the disabled server is not started at all
	if !a.mcp.Running("fake") || a.mcp.Running("off") || a.mcp.Running("broken") {
		t.Fatalf("running: %v", a.mcp.Names())
	}
	if n := a.lastNotice(); !strings.Contains(n, "broken") {
		t.Fatalf("a failing server must be reported, notice %q", n)
	}
	if r := a.st.VM.Exec(contextBackground(), "mcp.fake.echo({msg: 'hi'}).text"); r.Value != "echo:hi" || r.Err != "" {
		t.Fatalf("js call: %+v", r)
	}
	if got := a.systemPrompt(); !strings.Contains(got, "## MCP servers\n- fake (1 tool): echo") {
		t.Fatalf("tool missing from the prompt:\n%s", got)
	}
	a.openOverlay(OverMCP)
	a.draw()
	text := screenText(a.scr.(tcell.SimulationScreen))
	for _, want := range []string{"fake", "1 tools", "broken", "error", "off", "disabled"} {
		if !strings.Contains(text, want) {
			t.Fatalf("mcp dialog misses %q:\n%s", want, text)
		}
	}
}

func TestMCPHeadersFieldIsEditable(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.openMCPEdit(-1)
	var set func(string) error
	for _, f := range a.mcpFields() {
		if f.label == "headers" {
			set = f.set
		}
	}
	if set == nil {
		t.Fatal("no headers field")
	}
	if err := set("Authorization=Bearer abc, X-Team=core"); err != nil {
		t.Fatal(err)
	}
	if h := a.st.EditMCP.Headers; h["Authorization"] != "Bearer abc" || h["X-Team"] != "core" {
		t.Fatalf("headers %v", h)
	}
	if err := set(""); err != nil || a.st.EditMCP.Headers != nil {
		t.Fatalf("clearing: %v %v", err, a.st.EditMCP.Headers)
	}
}

func TestSystemPromptIncludesAgentsFiles(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	if err := os.MkdirAll(a.paths.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	g, p := agentsPaths(a.paths)
	os.WriteFile(g, []byte("be brief"), 0o644)
	os.WriteFile(p, []byte("use tabs"), 0o644)
	got := a.systemPrompt()
	for _, want := range []string{"## Global instructions\nFrom " + g, "be brief", "## Project instructions\nFrom " + p, "use tabs"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	os.Remove(p)
	os.Mkdir(p, 0o755) // unreadable as a file
	got = a.systemPrompt()
	if strings.Contains(got, "## Project instructions") || !strings.Contains(a.lastNotice(), "AGENTS.md") {
		t.Fatalf("unreadable AGENTS.md must be skipped with a notice (%q):\n%s", a.lastNotice(), got)
	}
}
