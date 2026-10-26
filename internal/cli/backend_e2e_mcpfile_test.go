package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meno/internal/client"
)

// fakeMCPHTTP is a streamable-http MCP server with one tool, "echo".
func fakeMCPHTTP(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     *int64 `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ID == nil {
			w.WriteHeader(202)
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-06-18"}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "echo", "description": "Echo.", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "from the file route"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(s.Close)
	return s
}

func waitStatus(t *testing.T, d *DaemonAPI, name, want string) string {
	t.Helper()
	got := ""
	for i := 0; i < 100; i++ {
		servers, err := d.MCPServers()
		if err != nil {
			t.Fatal(err)
		}
		got = "absent"
		for _, s := range servers {
			if s.Name == name {
				got = s.Status
			}
		}
		if got == want {
			return got
		}
		time.Sleep(50 * time.Millisecond)
	}
	return got
}

// mcpWarnings reads what `meno doctor` reads from the daemon: the warnings the
// configuration service reports for mcp.json.
func mcpWarnings(t *testing.T, home, socket string) []string {
	t.Helper()
	cl := client.New(home)
	cl.Socket = socket
	ms, err := cl.McpServers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return ms.Warnings
}

// An agent creates a server the way mcp-add help says: it writes an entry into
// $MENO_HOME/mcp.json and runs `meno mcp restart NAME`.
func TestAgentCreatesMCPServerByEditingTheFile(t *testing.T) {
	fake := fakeMCPHTTP(t)
	home, _, _, dapi, _ := startRig(t, &scripted{})
	entry := fmt.Sprintf(`[{"name":"made","transport":"streamable-http","url":%q}]`, fake.URL)
	if err := os.WriteFile(filepath.Join(home, "mcp.json"), []byte(entry), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := waitStatus(t, dapi, "made", "stopped"); got != "stopped" {
		t.Fatalf("the new entry should be listed as stopped before a restart, got %q", got)
	}
	res, err := dapi.MCPAdmin("restart", "made")
	if err != nil {
		t.Fatalf("restart of a server that only exists in the file: %v", err)
	}
	_ = res
	if got := waitStatus(t, dapi, "made", "running"); got != "running" {
		t.Fatalf("status %q, want running", got)
	}
	out, err := dapi.MCPCall("made", "echo", map[string]any{})
	if err != nil || out.Text != "from the file route" {
		t.Fatalf("call: %+v %v", out, err)
	}
}

func TestMCPFileWithABadEntryKeepsTheGoodOnes(t *testing.T) {
	fake := fakeMCPHTTP(t)
	home, _, _, dapi, socket := startRig(t, &scripted{})
	file := fmt.Sprintf(`[{"name":"Bad Name","transport":"streamable-http","url":%q},{"name":"good","transport":"streamable-http","url":%q}]`, fake.URL, fake.URL)
	if err := os.WriteFile(filepath.Join(home, "mcp.json"), []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := dapi.MCPAdmin("restart", "good"); err != nil {
		t.Fatal(err)
	}
	if got := waitStatus(t, dapi, "good", "running"); got != "running" {
		t.Fatalf("good server: %q", got)
	}
	if got := waitStatus(t, dapi, "Bad Name", "absent"); got != "absent" {
		t.Fatalf("the invalid entry is listed: %q", got)
	}
	if w := mcpWarnings(t, home, socket); len(w) != 1 || !strings.Contains(w[0], "Bad Name") {
		t.Fatalf("doctor would see warnings %q, want one naming the skipped entry", w)
	}
}

func TestMCPFileThatIsNotJSONIsReportedByDoctor(t *testing.T) {
	home, _, _, dapi, socket := startRig(t, &scripted{})
	if err := os.WriteFile(filepath.Join(home, "mcp.json"), []byte(`[{"name": "x",`), 0o600); err != nil {
		t.Fatal(err)
	}
	servers, err := dapi.MCPServers()
	if err != nil || len(servers) != 0 {
		t.Fatalf("servers=%+v err=%v", servers, err)
	}
	found := false
	for _, c := range configChecks(home) {
		found = found || (c.Name == "mcp.json" && c.Status == "error" && strings.Contains(c.Detail, "not valid JSON") && c.Fix != "")
	}
	if !found {
		t.Fatalf("doctor is silent about an unreadable mcp.json: %+v", configChecks(home))
	}
	if w := mcpWarnings(t, home, socket); len(w) == 0 {
		t.Fatal("the daemon reports no warning for an unreadable mcp.json")
	}
}

func TestMCPAddOfAnUnknownServerPointsToTheFile(t *testing.T) {
	_, _, _, dapi, _ := startRig(t, &scripted{})
	_, err := dapi.MCPAdmin("add", "nope")
	if err == nil {
		t.Fatal("add of an unknown server succeeded")
	}
	for _, want := range []string{`no MCP server "nope"`, "mcp.json", "meno mcp restart nope"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "use /mcp") {
		t.Fatalf("error still sends an agent to a dialog: %q", err)
	}
}
