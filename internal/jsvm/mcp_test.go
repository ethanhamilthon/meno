package jsvm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

type fakeHost struct {
	tools []MCPTool
	calls atomic.Int32
	last  map[string]any
}

func (f *fakeHost) Tools() []MCPTool { return f.tools }
func (f *fakeHost) Call(ctx context.Context, server, tool string, args map[string]any) (MCPResult, error) {
	f.calls.Add(1)
	f.last = args
	if tool == "boom" {
		return MCPResult{}, errors.New("it broke")
	}
	if tool == "wait" {
		<-ctx.Done()
		return MCPResult{}, ctx.Err()
	}
	b, _ := json.Marshal(map[string]any{"server": server, "tool": tool, "args": args})
	return MCPResult{Text: string(b), ResultID: "r1"}, nil
}
func (f *fakeHost) ResultInfo(id string) (any, error) { return map[string]any{"id": id}, nil }
func (f *fakeHost) ResultRead(id string, offset, limit int) (any, error) {
	return map[string]any{"id": id, "offset": offset, "limit": limit}, nil
}

func TestMCPToolsAreCallableFromJS(t *testing.T) {
	h := &fakeHost{tools: []MCPTool{
		{Server: "github", Name: "search", Description: "find", Schema: json.RawMessage(`{"type":"object"}`)},
		{Server: "my-db", Name: "run-query"},
		{Server: "github", Name: "boom"},
	}}
	v := New(Options{Root: t.TempDir()})
	if r := v.Exec(context.Background(), "typeof mcp + ' ' + mcpTools().length"); r.Value != "object 0" {
		t.Fatalf("without a host: %q %q", r.Value, r.Err)
	}
	v.SetMCP(h)
	r := run(t, v, "JSON.parse(mcp.github.search({q: 'x', n: 2}).text).args.q + ' ' + JSON.parse(mcp['my-db']['run-query']().text).tool + ' ' + JSON.parse(mcp.my_db.run_query({}).text).server")
	if r.Value != "x run-query my-db" {
		t.Fatalf("calls: %q", r.Value)
	}
	if h.last == nil || h.calls.Load() != 3 {
		t.Fatalf("calls %d last %v", h.calls.Load(), h.last)
	}
	r = run(t, v, "const t = mcpTools(); t.map(x => x.server + '.' + x.name).join(',') + ' ' + t[0].description + ' ' + t[0].schema.type")
	if r.Value != "github.search,my-db.run-query,github.boom find object" {
		t.Fatalf("mcpTools: %q", r.Value)
	}
	if r := v.Exec(context.Background(), "mcp.github.boom()"); !strings.Contains(r.Err, "it broke") {
		t.Fatalf("a failing tool must throw: %+v", r)
	}
	if r := v.Exec(context.Background(), "mcp.github.search('text')"); !strings.Contains(r.Err, "must be an object") {
		t.Fatalf("bad argument: %+v", r)
	}
	// Servers that start later show up on the next run.
	h.tools = append(h.tools, MCPTool{Server: "late", Name: "ping"})
	if r := run(t, v, "typeof mcp.late.ping"); r.Value != "function" {
		t.Fatalf("late server: %q", r.Value)
	}
}

func TestMCPCallsAreCancelable(t *testing.T) {
	h := &fakeHost{tools: []MCPTool{{Server: "s", Name: "wait"}}}
	v := New(Options{Root: t.TempDir()})
	v.SetMCP(h)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Result, 1)
	go func() { done <- v.Exec(ctx, "mcp.s.wait()") }()
	cancel()
	if r := <-done; r.Err == "" {
		t.Fatal("canceled call reported no error")
	}
	// The configured timeout limits a call as well.
	v.SetTimeout(200_000_000)
	if r := v.Exec(context.Background(), "mcp.s.wait()"); r.Err == "" {
		t.Fatal("timeout did not stop the call")
	}
}

func TestMCPEnvelopeShape(t *testing.T) {
	h := &fakeHost{tools: []MCPTool{{Server: "s", Name: "t"}}}
	v := New(Options{Root: t.TempDir()})
	v.SetMCP(h)
	r := run(t, v, "const r = mcp.s.t({}); [typeof r.text, r.structuredContent, Array.isArray(r.content), Array.isArray(r.artifacts), r.isError, r.resultId, r.truncated].join('|')")
	if r.Value != "string||true|true|false|r1|false" {
		t.Fatalf("envelope: %q", r.Value)
	}
	if r := run(t, v, "mcpResults.read('r1', {offset: 3, limit: 9}).offset + ':' + mcpResults.info('r1').id"); r.Value != "3:r1" {
		t.Fatalf("results api: %q", r.Value)
	}
}
