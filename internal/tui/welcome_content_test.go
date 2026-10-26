package tui

import (
	"strings"
	"testing"

	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/render"
)

func welcomeText(t *testing.T, a *App) string {
	t.Helper()
	a.draw()
	return screenTextOf(a)
}

func TestWelcomeHintsAreCurrent(t *testing.T) {
	a, _ := visualFixture(t, 112, 36)
	out := welcomeText(t, a)
	for _, want := range []string{"commands", "@", "files", "ctrl+v", "image", "/help", "all keys"} {
		if !strings.Contains(out, want) {
			t.Fatalf("welcome hints miss %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "write") {
		t.Fatalf("welcome still names the removed insert mode:\n%s", out)
	}
}

func TestWelcomeTagline(t *testing.T) {
	a, _ := visualFixture(t, 112, 36)
	if !strings.Contains(welcomeText(t, a), "thin and efficient AI coding harness") {
		t.Fatal("tagline missing")
	}
}

func TestWelcomeMarksFailedMCP(t *testing.T) {
	a, _ := visualFixture(t, 160, 36)
	a.st.Servers = []mcp.Server{{Name: "a", Enabled: true}, {Name: "b", Enabled: true}, {Name: "off"}}
	a.st.ServerInfo = map[string]mcpInfo{"a": {Running: true}, "b": {Err: "boom"}}
	out := welcomeText(t, a)
	if !strings.Contains(out, "2 mcp") || !strings.Contains(out, "1 failed") {
		t.Fatalf("want '2 mcp' and '1 failed':\n%s", out)
	}
	a.st.ServerInfo = map[string]mcpInfo{"a": {Running: true}, "b": {Running: true}}
	if out = welcomeText(t, a); strings.Contains(out, "failed") {
		t.Fatalf("no server failed, but the welcome says so:\n%s", out)
	}
}

func TestWelcomePointsToLoginWithoutProvider(t *testing.T) {
	a, _ := visualFixture(t, 160, 36)
	a.st.Settings.Model = ""
	if s := a.st.Runtime.Session; s != nil {
		s.Provider, s.Model, s.Effort = "", "", ""
	}
	a.st.Settings.Effort = ""
	a.st.Providers = nil
	if out := welcomeText(t, a); !strings.Contains(out, "/login") {
		t.Fatalf("no provider: want /login:\n%s", out)
	}
	a.st.Providers = []llm.Provider{{Name: "p", BaseURL: "http://127.0.0.1:1/v1", Protocol: llm.ProtoResponses}}
	if out := welcomeText(t, a); !strings.Contains(out, "/model") || strings.Contains(out, "/login") {
		t.Fatalf("provider works but no model: want /model only:\n%s", out)
	}
}

// The info row is cut at the gutters, so it must be chosen to fit inside them:
// a group that would be clipped is dropped instead.
func TestWelcomeInfoLineFitsInsideGutters(t *testing.T) {
	a, _ := visualFixture(t, 80, 24)
	a.paths.Cwd = "/Users/someone/projects/a-rather-long-project-name"
	a.st.Cwd = a.paths.Cwd
	for w := 40; w <= 160; w++ {
		if got := segsWidth(a.infoLine(w)); got > w-2*render.Gutter && len(a.infoLine(w)) > 1 {
			t.Fatalf("width %d: info row is %d cells, room is %d", w, got, w-2*render.Gutter)
		}
	}
}
