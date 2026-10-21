package llm

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestExportToolDefinitionMatchesWireSchema(t *testing.T) {
	for _, name := range []string{"", "js", "bash"} {
		req := Request{Tools: true, ToolName: name, ToolDescription: "effective description", Provider: Provider{APIKey: "SECRET"}}
		var got map[string]any
		if err := json.Unmarshal(req.ExportToolDefinition(), &got); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(req.toolSchema())
		var want map[string]any
		_ = json.Unmarshal(raw, &want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s export differs from wire schema", name)
		}
		req.Tools = false
		if req.ExportToolDefinition() != nil {
			t.Fatal("disabled tools exported")
		}
	}
}
