package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A Python server that writes to a pipe block-buffers its output. Without an
// unbuffered stdout the handshake never gets an answer and the client waits
// until it gives up.
const bufferedServer = `import sys, json
for line in sys.stdin:
    m = json.loads(line)
    if "id" not in m:
        continue
    meth = m.get("method")
    if meth == "initialize":
        r = {"protocolVersion": "2025-06-18", "capabilities": {}, "serverInfo": {"name": "fake", "version": "1"}}
    elif meth == "tools/list":
        r = {"tools": [{"name": "echo", "description": "echo", "inputSchema": {"type": "object"}}]}
    else:
        r = {}
    sys.stdout.write(json.dumps({"jsonrpc": "2.0", "id": m["id"], "result": r}) + "\n")
`

func writeScript(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStdioServerAnswersWithUnbufferedStdout(t *testing.T) {
	p := writeScript(t, "buffered.py", bufferedServer)
	s := Server{Name: "buffered", Transport: TransportStdio, Command: "python3 " + p, Enabled: true}
	c, err := Start(context.Background(), s)
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	defer c.Close()
	if tools := c.ToolSnapshot(); len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools %+v", tools)
	}
}

func TestStdioServerUsesArgsAndCwd(t *testing.T) {
	dir := t.TempDir()
	p := writeScript(t, "cwd.py", "import os, sys, json\n"+
		"cwd = os.getcwd()\n"+
		"for line in sys.stdin:\n"+
		"    m = json.loads(line)\n"+
		"    if 'id' not in m:\n"+
		"        continue\n"+
		"    meth = m.get('method')\n"+
		"    if meth == 'initialize':\n"+
		"        r = {'protocolVersion': '2025-06-18', 'capabilities': {}, 'serverInfo': {'name': 'fake', 'version': '1'}}\n"+
		"    elif meth == 'tools/list':\n"+
		"        r = {'tools': [{'name': cwd, 'inputSchema': {'type': 'object'}}]}\n"+
		"    else:\n"+
		"        r = {}\n"+
		"    sys.stdout.write(json.dumps({'jsonrpc': '2.0', 'id': m['id'], 'result': r}) + '\\n')\n"+
		"    sys.stdout.flush()\n")
	s := Server{Name: "withargs", Transport: TransportStdio, Command: "python3", Args: []string{p}, Cwd: dir, Enabled: true}
	c, err := Start(context.Background(), s)
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	defer c.Close()
	tools := c.ToolSnapshot()
	if len(tools) != 1 {
		t.Fatalf("tools %+v", tools)
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		resolved = dir
	}
	if tools[0].Name != resolved {
		t.Fatalf("server ran in %q, want %q", tools[0].Name, resolved)
	}
}

func TestStdioEnvironmentIsUnbufferedByDefault(t *testing.T) {
	env := stdioEnv(Server{Name: "x", Command: "python3"})
	if !hasEnv(env, "PYTHONUNBUFFERED=1") {
		t.Fatalf("PYTHONUNBUFFERED not set: %v", env)
	}
	env = stdioEnv(Server{Name: "x", Command: "python3", Env: map[string]string{"PYTHONUNBUFFERED": "0"}})
	if hasEnv(env, "PYTHONUNBUFFERED=1") {
		t.Fatal("the server's own value must win")
	}
}

func hasEnv(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}

func TestStdioStartupErrorCarriesStderr(t *testing.T) {
	p := writeScript(t, "boom.py", "import sys\nsys.stderr.write('inventory: cannot load config\\n')\nsys.exit(3)\n")
	s := Server{Name: "boom", Transport: TransportStdio, Command: "python3 " + p, Enabled: true}
	_, err := Start(context.Background(), s)
	if err == nil {
		t.Fatal("the server was accepted")
	}
	if !strings.Contains(err.Error(), "cannot load config") {
		t.Fatalf("stderr is missing: %v", err)
	}
}

func TestStartupTimeoutNamesTheStep(t *testing.T) {
	p := writeScript(t, "silent.py", "import sys\nfor line in sys.stdin:\n    pass\n")
	s := Server{Name: "silent", Transport: TransportStdio, Command: "python3 " + p, Enabled: true}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := Start(ctx, s)
	if err == nil {
		t.Fatal("the silent server was accepted")
	}
	if !strings.Contains(err.Error(), "initialize") {
		t.Fatalf("the error does not name the step: %v", err)
	}
}
