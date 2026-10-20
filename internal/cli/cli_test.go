package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeAPI struct {
	servers   []Server
	tools     []Tool
	args      map[string]any
	start     TaskStartRequest
	wait      TaskWait
	forbidden bool
}

func (f *fakeAPI) check() error {
	if f.forbidden {
		return &APIError{Code: "forbidden", Message: "not allowed"}
	}
	return nil
}
func (f *fakeAPI) MCPServers() ([]Server, error)   { return f.servers, f.check() }
func (f *fakeAPI) MCPTools(string) ([]Tool, error) { return f.tools, f.check() }
func (f *fakeAPI) MCPCall(_, _ string, a map[string]any) (MCPResult, error) {
	f.args = a
	return MCPResult{Text: "ok"}, f.check()
}
func (f *fakeAPI) MCPResults(string, int, int) (MCPPage, error) {
	return MCPPage{Text: "page"}, f.check()
}
func (f *fakeAPI) MCPAdmin(_, s string) (Server, error) { return Server{Name: s}, f.check() }
func (f *fakeAPI) TasksStart(r TaskStartRequest) (Task, error) {
	f.start = r
	return Task{ID: "t1", Status: "running"}, f.check()
}
func (f *fakeAPI) TasksList(bool) ([]Task, error)                   { return nil, f.check() }
func (f *fakeAPI) TaskStatus(id string) (Task, error)               { return Task{ID: id}, f.check() }
func (f *fakeAPI) TaskLogs(string, int, int) (TaskLogs, error)      { return TaskLogs{}, f.check() }
func (f *fakeAPI) TaskWait(string, time.Duration) (TaskWait, error) { return f.wait, f.check() }
func (f *fakeAPI) TaskCancel(id string) (Task, error)               { return Task{ID: id}, f.check() }
func (f *fakeAPI) Ask([]byte) (map[string]any, error) {
	return map[string]any{"answer": "yes"}, f.check()
}
func (f *fakeAPI) GoalDone(string) error    { return f.check() }
func (f *fakeAPI) GoalBlocked(string) error { return f.check() }
func (f *fakeAPI) WebSearch(q string, n int) (WebResult, error) {
	return WebResult{Query: q}, f.check()
}
func (f *fakeAPI) View(p string) (ViewResult, error) {
	return ViewResult{Path: p, Displayed: true}, f.check()
}
func (f *fakeAPI) Models(string, bool) ([]Model, error) { return nil, f.check() }
func (f *fakeAPI) Daemon(string, bool) (DaemonStatus, error) {
	return DaemonStatus{Running: true}, f.check()
}
func (f *fakeAPI) Sessions(string) ([]Session, error)     { return nil, f.check() }
func (f *fakeAPI) Send(r SendRequest) (SendResult, error) { return SendResult{Text: r.Text}, f.check() }
func (f *fakeAPI) Tail(string) ([]Event, error)           { return nil, f.check() }
func run(args []string, a API, input string) (int, string, string) {
	var out, errOut bytes.Buffer
	c := Run(args, IO{In: strings.NewReader(input), Out: &out, Err: &errOut, Dir: "."}, a)
	return c, out.String(), errOut.String()
}
func TestReadOutputAndPerformance(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "a")
	_ = os.WriteFile(p, []byte("one\ntwo\n"), 0644)
	start := time.Now()
	c, out, e := run([]string{"read", p, "2", "2", "--hash"}, nil, "")
	if c != 0 || e != "" || !strings.HasPrefix(out, "2\ttwo\nsha256 ") {
		t.Fatalf("%d %q %q", c, out, e)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Fatalf("read: %s", time.Since(start))
	}
}
func TestWriteEditAndAtomicJSON(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "a")
	c, out, _ := run([]string{"write", p}, nil, "a\nb\n")
	if c != 0 || !strings.Contains(out, "2 lines") {
		t.Fatalf("%d %s", c, out)
	}
	c, _, e := run([]string{"write", p}, nil, "secret")
	if c != 1 || !strings.Contains(e, "already exists") || !strings.Contains(e, "sha256") {
		t.Fatalf("%d %s", c, e)
	}
	hash := hashBytes([]byte("a\nb\n"))
	c, _, _ = run([]string{"edit", p, "--json", "--expect-hash", hash}, nil, "[{\"oldText\":\"a\",\"newText\":\"A\"},{\"oldText\":\"missing\",\"newText\":\"x\"}]")
	b, _ := os.ReadFile(p)
	if c != 1 || string(b) != "a\nb\n" {
		t.Fatalf("non-atomic edit: %q", b)
	}
	c, _, _ = run([]string{"edit", p, "--json", "--expect-hash", hash}, nil, "[{\"oldText\":\"a\",\"newText\":\"A\"}]")
	b, _ = os.ReadFile(p)
	if c != 0 || string(b) != "A\nb\n" {
		t.Fatalf("JSON edit: %d %q", c, b)
	}
	c, _, e = run([]string{"edit", p, "--json", "--expect-hash"}, nil, "")
	if c != 2 || !strings.Contains(e, "needs a value") {
		t.Fatalf("missing hash: %d %s", c, e)
	}
	c, _, e = run([]string{"edit", p, "b\nwrong", "X"}, nil, "")
	if c != 1 || !strings.Contains(e, "stopped matching at line 3") {
		t.Fatalf("%d %s", c, e)
	}
	code, _, _ := run([]string{"edit", p, "--json", "--expect-hash", hash}, nil, "[{\"oldText\":\"A\",\"newText\":\"B\"}]")
	if code != 1 {
		t.Fatalf("stale JSON edit code %d", code)
	}
}
func TestSearchAndMCPArgumentForms(t *testing.T) {
	d := t.TempDir()
	_ = os.Mkdir(filepath.Join(d, "sub"), 0755)
	_ = os.WriteFile(filepath.Join(d, "sub", "a.go"), []byte("needle\n"), 0644)
	c, out, e := run([]string{"search", "nee.*", filepath.Join(d, "sub"), "--regex", "--glob", "*.go", "--json"}, nil, "")
	if c != 0 || e != "" || !strings.Contains(out, "\"line\":1") {
		t.Fatalf("%d %s %s", c, out, e)
	}
	a := &fakeAPI{servers: []Server{{Name: "orders"}}, tools: []Tool{{Name: "get_order"}}}
	for _, args := range [][]string{{"mcp", "call", "orders", "get_order", "id=7", "active=true"}, {"mcp", "call", "orders.get_order", "{\"id\":7,\"active\":true}"}, {"mcp", "orders.get_order", "id=7"}} {
		c, _, e = run(args, a, "")
		if c != 0 || e != "" {
			t.Fatalf("%v: %d %s", args, c, e)
		}
	}
	if a.args["id"] != float64(7) {
		t.Fatalf("%#v", a.args)
	}
	c, _, e = run([]string{"mcp", "call", "orders", "get_order", "id=7", "active=true"}, a, "")
	if a.args["active"] != true {
		t.Fatalf("key=value bool: %#v", a.args)
	}
	c, _, e = run([]string{"mcp", "call", "ordres.get_order"}, a, "")
	if c != 1 || !strings.Contains(e, "did you mean") {
		t.Fatalf("%d %s", c, e)
	}
}
func TestTasksAndExitCodes(t *testing.T) {
	var e string
	a := &fakeAPI{wait: TaskWait{Task: Task{ID: "t1"}, TimedOut: true}}
	c, _, _ := run([]string{"tasks", "start", "echo", "hi", "--cwd", "/tmp", "--env", "A=2", "--env", "B=3", "--timeout", "10"}, a, "")
	if c != 0 || a.start.Command != "echo hi" || a.start.Env["A"] != "2" || a.start.Env["B"] != "3" {
		t.Fatalf("%d %#v", c, a.start)
	}
	c, _, _ = run([]string{"tasks", "wait", "t1"}, a, "")
	if c != 124 {
		t.Fatalf("timeout code %d", c)
	}
	a.wait.TimedOut = false
	a.forbidden = true
	a.forbidden = false
	for _, tc := range []struct {
		args []string
		want int
		hint string
	}{{[]string{"taks"}, 1, "did you mean \"tasks\""}, {[]string{"tasks", "statuus"}, 1, "did you mean \"status\""}, {[]string{"tasks", "start", "echo", "--timout", "1"}, 1, "did you mean \"--timeout\""}, {[]string{"tasks"}, 2, "usage:"}} {
		c, _, e := run(tc.args, a, "")
		if c != tc.want || !strings.Contains(e, tc.hint) {
			t.Errorf("%v: %d %s", tc.args, c, e)
		}
	}
	a.forbidden = true
	c, _, e = run([]string{"tasks", "list"}, a, "")
	if c != 3 || !strings.Contains(e, "not allowed") {
		t.Fatalf("%d %s", c, e)
	}
}
func TestHelpGroupsAndLocalHelp(t *testing.T) {
	for _, name := range []string{"mcp", "tasks", "ask", "goal", "web", "view", "models", "daemon", "sessions", "send", "tail", "read", "edit", "write", "search"} {
		c, out, e := run([]string{name, "--help"}, nil, "")
		if c != 0 || e != "" || out == "" {
			t.Errorf("%s: %d %q", name, c, e)
		}
	}
}
