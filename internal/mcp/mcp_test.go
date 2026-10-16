package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeServer = `import sys, json
for line in sys.stdin:
    m = json.loads(line)
    if "id" not in m:
        continue
    meth = m.get("method")
    if meth == "initialize":
        r = {"protocolVersion": "2025-06-18", "capabilities": {}, "serverInfo": {"name": "fake", "version": "1"}}
    elif meth == "tools/list":
        r = {"tools": [{"name": "echo", "description": "echo", "inputSchema": {"type": "object"}}]}
    elif meth == "tools/call":
        a = m["params"]["arguments"]
        if m["params"]["name"] == "fail":
            r = {"content": [{"type": "text", "text": "boom"}], "isError": True}
        else:
            r = {"content": [{"type": "text", "text": "echo:" + str(a.get("msg"))}]}
    else:
        r = {}
    sys.stdout.write(json.dumps({"jsonrpc": "2.0", "id": m["id"], "result": r}) + "\n")
    sys.stdout.flush()
`

func fakeCmd(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake.py")
	if err := os.WriteFile(p, []byte(fakeServer), 0o644); err != nil {
		t.Fatal(err)
	}
	return "python3 " + p
}

func TestStdioHandshakeAndCall(t *testing.T) {
	s := Server{Name: "fake", Transport: TransportStdio, Command: fakeCmd(t), Enabled: true}
	c, err := Start(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if len(c.Tools) != 1 || c.Tools[0].Name != "echo" {
		t.Fatalf("tools: %+v", c.Tools)
	}
	out, err := c.Call(context.Background(), "echo", map[string]any{"msg": "hi"})
	if err != nil || out != "echo:hi" {
		t.Fatalf("call: %q %v", out, err)
	}
	if _, err := c.Call(context.Background(), "fail", nil); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("isError not surfaced: %v", err)
	}
}

func TestManagerLifecycle(t *testing.T) {
	m := NewManager()
	s := Server{Name: "fake", Transport: TransportStdio, Command: fakeCmd(t), Enabled: true}
	if err := m.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if !m.Running("fake") || m.ToolCount() != 1 {
		t.Fatal("not running")
	}
	m.Stop("fake")
	if m.Running("fake") {
		t.Fatal("still running after stop")
	}
}

func TestManagerRejectsInvalidAndRecordsError(t *testing.T) {
	m := NewManager()
	err := m.Start(context.Background(), Server{Name: "Bad Name", Transport: TransportStdio, Command: "x"})
	if err == nil || m.Err("Bad Name") == "" {
		t.Fatal("invalid name accepted")
	}
	err = m.Start(context.Background(), Server{Name: "nobin", Transport: TransportStdio, Command: "/definitely/not/here"})
	if err == nil || m.Err("nobin") == "" {
		t.Fatal("missing binary not reported")
	}
}

func TestValidateServer(t *testing.T) {
	ok := Server{Name: "a-1", Transport: TransportStdio, Command: "x"}
	if ValidateServer(ok) != nil {
		t.Fatal("valid rejected")
	}
	bad := []Server{
		{Name: "", Transport: TransportStdio, Command: "x"},
		{Name: "a", Transport: TransportStdio},
		{Name: "a", Transport: TransportSSE},
		{Name: "a", Transport: "ws", URL: "x"},
		{Name: strings.Repeat("a", 41), Transport: TransportStdio, Command: "x"},
	}
	for i, s := range bad {
		if ValidateServer(s) == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}

func TestExtractTextIgnoresNonText(t *testing.T) {
	s, err := ExtractText([]byte(`{"content":[{"type":"image","text":"x"},{"type":"text","text":"ok"}]}`))
	if err != nil || s != "ok" {
		t.Fatalf("%q %v", s, err)
	}
}
