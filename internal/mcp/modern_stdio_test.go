package mcp

import (
	"context"
	"testing"
	"time"
)

// eraScript is a stdio server. MODE=modern answers server/discover and has no
// initialize; MODE=legacy answers initialize and errors on anything else;
// MODE=silent ignores unknown methods before initialize; MODE=dual does both.
const eraScript = `import sys, json, os
mode = os.environ.get("MODE")
def out(i, r=None, e=None):
    m = {"jsonrpc": "2.0", "id": i}
    if e: m["error"] = e
    else: m["result"] = r
    sys.stdout.write(json.dumps(m) + "\n"); sys.stdout.flush()
for line in sys.stdin:
    m = json.loads(line)
    if "id" not in m: continue
    meth = m.get("method")
    meta = (m.get("params") or {}).get("_meta") or {}
    modern = "io.modelcontextprotocol/protocolVersion" in meta
    if modern and mode in ("modern", "dual"):
        if meth == "server/discover":
            out(m["id"], {"resultType": "complete", "supportedVersions": ["2026-07-28"], "capabilities": {"tools": {}}})
        elif meth == "tools/list":
            out(m["id"], {"resultType": "complete", "tools": [{"name": "echo", "inputSchema": {"type": "object"}}]})
        elif meth == "tools/call":
            out(m["id"], {"resultType": "complete", "content": [{"type": "text", "text": "modern"}]})
    elif meth == "initialize" and mode in ("legacy", "dual", "silent"):
        out(m["id"], {"protocolVersion": "2025-06-18", "capabilities": {}, "serverInfo": {"name": "f"}})
    elif meth == "tools/list" and mode != "modern":
        out(m["id"], {"tools": [{"name": "echo", "inputSchema": {"type": "object"}}]})
    elif mode == "legacy":
        out(m["id"], e={"code": -32602, "message": "expected initialize"})
    elif mode == "modern":
        out(m["id"], e={"code": -32601, "message": "unknown"})
`

func startStdioEra(t *testing.T, mode, protocol string) (*Client, error) {
	t.Helper()
	old := probeTimeout
	probeTimeout = 700 * time.Millisecond
	t.Cleanup(func() { probeTimeout = old })
	p := writeScript(t, "era.py", eraScript)
	return Start(context.Background(), Server{Name: "s", Transport: TransportStdio, Command: "python3", Args: []string{p}, Env: map[string]string{"MODE": mode}, Enabled: true, Protocol: protocol})
}

func TestModernStdio(t *testing.T) {
	c, err := startStdioEra(t, "modern", ProtocolModern)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	res, err := c.CallResult(context.Background(), "echo", nil)
	if err != nil || res.Text != "modern" || !c.modern {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestAutoStdioFallsBackToLegacy(t *testing.T) {
	for _, mode := range []string{"legacy", "silent"} {
		c, err := startStdioEra(t, mode, ProtocolAuto)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if c.modern || len(c.ToolSnapshot()) != 1 {
			t.Fatalf("%s: modern=%v tools=%v", mode, c.modern, c.ToolSnapshot())
		}
		c.Close()
	}
}

func TestAutoStdioPrefersModernOnDualEra(t *testing.T) {
	c, err := startStdioEra(t, "dual", ProtocolAuto)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !c.modern {
		t.Fatal("dual-era must be modern")
	}
}

func TestExplicitLegacyStdioIgnoresModern(t *testing.T) {
	c, err := startStdioEra(t, "dual", ProtocolLegacy)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.modern {
		t.Fatal("explicit legacy must stay legacy")
	}
}
