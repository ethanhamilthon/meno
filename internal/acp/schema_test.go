package acp

import (
	"bytes"
	"encoding/json"
	"meno/internal/llm"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Official stable v1 schema pinned at upstream commit
// 097a8a8e9f79998b56bff79ac3bb125b775e3c50 (checked 2026-10-24).
// Validation is offline and includes actual stdio session updates, not just
// method registration. Permissions are deliberately outside Meno's surface.
func validateSchema(t *testing.T, definition string, value any) {
	t.Helper()
	data, err := os.ReadFile("testdata/acp-v1-097a8a8.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err = c.AddResource("https://meno.test/acp-v1.json", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile("https://meno.test/acp-v1.json#/$defs/" + definition)
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := jsonschema.UnmarshalJSON(bytes.NewReader(mustJSON(value)))
	if err != nil {
		t.Fatal(err)
	}
	if err = schema.Validate(normalized); err != nil {
		t.Fatalf("%s: %v", definition, err)
	}
}
func TestPinnedSchemaWireSurface(t *testing.T) {
	c, _ := daemonRig(t, &gated{steps: [][]llm.Event{{bashCall("schema-tool", "printf sandbox"), {Kind: llm.EvDone}}, {{Kind: llm.EvThinking, Text: "reason"}, {Kind: llm.EvText, Text: "answer"}, {Kind: llm.EvDone}}}})
	p := startPeer(t, c)
	init := result(t, p.call("initialize", map[string]any{"protocolVersion": 1, "clientCapabilities": map[string]any{}}))
	validateSchema(t, "InitializeResponse", init)
	created := result(t, p.call("session/new", map[string]any{"cwd": t.TempDir()}))
	validateSchema(t, "NewSessionResponse", created)
	id := created["sessionId"].(string)
	validateSchema(t, "PromptResponse", promptText(t, p, id, "schema test"))
	validateSchema(t, "ListSessionsResponse", result(t, p.call("session/list", map[string]any{})))
	p.mu.Lock()
	notes := append([]message(nil), p.notes...)
	p.mu.Unlock()
	for _, note := range notes {
		if note.Method == "session/update" {
			var v any
			_ = json.Unmarshal(note.Params, &v)
			validateSchema(t, "SessionNotification", v)
		}
	}
}
