package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
)

func TestTurnSystemRoundTripsThroughTheDatabase(t *testing.T) {
	dir := t.TempDir()
	s := agent.NewSession("/w", "m")
	s.Append(agent.Turn{Role: llm.RoleUser, Text: "hello", System: "<system-event>x</system-event>"})
	s.Append(agent.Turn{Role: llm.RoleAssistant, Text: "hi"})
	if err := agent.SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	back, err := agent.LoadSession(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Snapshot()[0].System != "<system-event>x</system-event>" || back.Title != "hello" {
		t.Fatalf("%+v title=%q", back.Snapshot()[0], back.Title)
	}
	msgs := back.ToMsgs()
	if msgs[0].Text != "<system-event>x</system-event>\n\nhello" {
		t.Fatalf("msgs[0] = %q", msgs[0].Text)
	}
	agent.CloseSessionStores()
}

func TestPreparationDoesNotBlockTheUI(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	release := make(chan struct{})
	a.prepareHook = func() { <-release }
	typeRunes(a, "slow start")
	start := time.Now()
	a.onKey(keyK(tcell.KeyEnter))
	if d := time.Since(start); d > time.Second {
		t.Fatalf("submit blocked the UI for %s", d)
	}
	if !a.st.Running || a.st.Phase != "preparing" {
		t.Fatalf("running=%v phase=%q", a.st.Running, a.st.Phase)
	}
	// The event loop keeps handling keys and drawing meanwhile.
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "preparing...") {
		t.Fatalf("screen while preparing:\n%s", text)
	}
	typeRunes(a, "next")
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "next") {
		t.Fatalf("typing while preparing:\n%s", text)
	}
	close(release)
	pump(t, a, func() bool { return !a.st.Running })
	if len(cl.requests()) != 1 {
		t.Fatal("request not sent")
	}
}

func TestDiffBaselineKinds(t *testing.T) {
	mk := func() *baseline {
		return &baseline{
			Cwd:    "/a",
			Skills: map[string]skillSig{"B": {"/p/B.md", "h1"}, "D": {"/p/D.md", "h"}},
			Agents: map[string]fileSig{"global": {}, "project": {Path: "/a/AGENTS.md", Hash: "x", Exists: true}},
			MCP:    map[string]mcpSig{"gh": {State: "running", Tools: []string{"mcp.gh.old()"}}},
			Date:   "2026-10-17",
		}
	}
	cur := mk()
	cur.Skills["B"] = skillSig{"/p/B.md", "h2"}
	cur.Skills["C"] = skillSig{"/p/C.md", "h"}
	delete(cur.Skills, "D")
	cur.Agents["project"] = fileSig{Path: "/a/AGENTS.md", Hash: "y", Exists: true}
	cur.MCP["gh"] = mcpSig{State: "running", Tools: []string{"mcp.gh.search({q: string})"}}
	cur.MCP["db"] = mcpSig{State: "failed", Err: "boom"}
	cur.Date = "2026-10-18"
	cur.Cwd = "/b"
	got := strings.Join(diffBaseline(mk(), cur), "\n")
	for _, want := range []string{
		"<skills-updated>Skill \"B\" was updated: reread /p/B.md before you use it.</skills-updated>",
		"<skills-added>Skill \"C\" is now available: /p/C.md</skills-added>",
		"<skills-removed>Skill \"D\" is no longer available.</skills-removed>",
		"<agents-md-updated>Project instructions changed: reread /a/AGENTS.md.</agents-md-updated>",
		"<mcp-updated>Server \"gh\" changed its tools. New tools: mcp.gh.search({q: string}). Removed tools: mcp.gh.old().</mcp-updated>",
		"<mcp-updated>Server \"db\" failed: boom</mcp-updated>",
		"<date-changed>The date is now 2026-10-18, Sunday.</date-changed>",
		"<cwd-changed>The working directory changed from /a to /b. Relative paths and sh() now use /b.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if len(diffBaseline(mk(), mk())) != 0 || len(diffBaseline(nil, cur)) != 0 {
		t.Fatal("equal baselines give no events")
	}
	// Server added, then removed.
	gone := mk()
	delete(gone.MCP, "gh")
	if got := strings.Join(diffBaseline(mk(), gone), "\n"); !strings.Contains(got, "Server \"gh\" is no longer running") {
		t.Fatal(got)
	}
	conn := mk()
	conn.MCP["new"] = mcpSig{State: "running", Tools: []string{"mcp.new.x()"}}
	if got := strings.Join(diffBaseline(mk(), conn), "\n"); !strings.Contains(got, "Server \"new\" connected. New tools: mcp.new.x().") {
		t.Fatal(got)
	}
	if b := eventBlock(nil); b != "" {
		t.Fatal("no lines, no block")
	}
}

func TestCwdChangeKeepsPromptAndReportsEverything(t *testing.T) {
	// :cd arrives in stage 3; the mechanism is a diff between two worlds.
	old := &baseline{Cwd: "/a", Skills: map[string]skillSig{}, Agents: map[string]fileSig{"project": {Path: "/a/AGENTS.md", Hash: "x", Exists: true}}, MCP: map[string]mcpSig{}}
	cur := &baseline{Cwd: "/b", Skills: map[string]skillSig{"s": {"/b/.meno/skills/s.md", "h"}}, Agents: map[string]fileSig{"project": {Path: "/b/AGENTS.md", Hash: "y", Exists: true}}, MCP: map[string]mcpSig{}}
	got := strings.Join(diffBaseline(old, cur), "\n")
	for _, want := range []string{"<cwd-changed>", "<skills-added>Skill \"s\"", "reread /b/AGENTS.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}
