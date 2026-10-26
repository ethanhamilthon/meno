package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func toolsAPI() *fakeAPI {
	return &fakeAPI{
		servers: []Server{{Name: "orders", Status: "running"}},
		tools: []Tool{
			{Name: "get_order", Description: "Fetch one order.\n\n      Best for: lookups by id.\n      Returns: the order as JSON.", InputSchema: map[string]any{
				"type": "object", "required": []any{"id"}, "properties": map[string]any{"id": map[string]any{"type": "integer"}}}},
			{Name: "ping", Description: "  Check the server.  "},
			{Name: "bare"},
		},
	}
}

func TestMCPToolsListIsOneLinePerTool(t *testing.T) {
	c, out, e := run([]string{"mcp", "tools", "orders"}, toolsAPI(), "")
	if c != 0 || e != "" {
		t.Fatalf("%d %s", c, e)
	}
	want := "get_order — Fetch one order.\nping — Check the server.\nbare\n"
	if out != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func TestMCPToolsShowsOneToolWithSchema(t *testing.T) {
	c, out, e := run([]string{"mcp", "tools", "orders", "get_order"}, toolsAPI(), "")
	if c != 0 || e != "" {
		t.Fatalf("%d %s", c, e)
	}
	for _, want := range []string{"get_order\n", "Fetch one order.\n\nBest for: lookups by id.\nReturns: the order as JSON.\n", "Input schema:\n", `"required": [`, `"type": "integer"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "      Best for") {
		t.Fatalf("description keeps the server's indentation:\n%s", out)
	}
}

func TestMCPToolWithoutSchemaSaysSo(t *testing.T) {
	_, out, _ := run([]string{"mcp", "tools", "orders", "bare"}, toolsAPI(), "")
	if !strings.Contains(out, "takes no declared arguments") {
		t.Fatalf("%q", out)
	}
}

func TestMCPToolsUnknownToolHintsAndFails(t *testing.T) {
	c, _, e := run([]string{"mcp", "tools", "orders", "get_ordr"}, toolsAPI(), "")
	if c == 0 || !strings.Contains(e, "get_order") || !strings.Contains(e, "did you mean") {
		t.Fatalf("%d %q", c, e)
	}
}

func TestMCPToolsJSON(t *testing.T) {
	_, out, _ := run([]string{"mcp", "tools", "orders", "--json"}, toolsAPI(), "")
	var list []Tool
	if err := json.Unmarshal([]byte(out), &list); err != nil || len(list) != 3 || list[0].InputSchema == nil || !strings.Contains(list[0].Description, "Best for") {
		t.Fatalf("list json changed: %v %q", err, out)
	}
	_, out, _ = run([]string{"mcp", "tools", "orders", "get_order", "--json"}, toolsAPI(), "")
	var one Tool
	if err := json.Unmarshal([]byte(out), &one); err != nil || one.Name != "get_order" || one.InputSchema == nil {
		t.Fatalf("single json: %v %q", err, out)
	}
}

func TestMCPToolsTooManyArguments(t *testing.T) {
	if c, _, e := run([]string{"mcp", "tools", "orders", "a", "b"}, toolsAPI(), ""); c != ExitUsage || !strings.Contains(e, "usage: meno mcp tools SERVER [TOOL]") {
		t.Fatalf("%d %q", c, e)
	}
}
