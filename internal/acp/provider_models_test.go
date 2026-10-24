package acp

import (
	"encoding/json"
	"testing"
)

func TestAvailableModelsComeOnlyFromConfiguredProviders(t *testing.T) {
	c, _ := daemonRig(t, &gated{})
	p := startPeer(t, c)
	p.call("initialize", map[string]any{"protocolVersion": 1})
	r := result(t, p.call("session/new", map[string]any{"cwd": t.TempDir()}))
	raw, err := json.Marshal(r["models"])
	if err != nil {
		t.Fatal(err)
	}
	var models struct {
		Available []struct {
			ID string `json:"modelId"`
		} `json:"availableModels"`
	}
	if err := json.Unmarshal(raw, &models); err != nil {
		t.Fatal(err)
	}
	if len(models.Available) != 1 || models.Available[0].ID != "test/fake" {
		t.Fatalf("models: %s", raw)
	}
}
