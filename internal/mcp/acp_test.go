package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestACPTransportMappingAndValidation(t *testing.T) {
	for _, raw := range []string{
		`{"name":"stdio","command":"/tmp/executable with space","args":["a"],"env":[{"name":"SECRET","value":"credential"}]}`,
		`{"type":"http","name":"http","url":"http://127.0.0.1/mcp","headers":[{"name":"Authorization","value":"credential"}]}`,
		`{"type":"sse","name":"sse","url":"http://127.0.0.1/sse","headers":[]}`,
	} {
		out, err := ParseACPServers([]json.RawMessage{json.RawMessage(raw)}, "/tmp")
		if err != nil || len(out) != 1 {
			t.Fatal(err)
		}
		if out[0].Transport == TransportStdio {
			parts, err := stdioCommand(out[0])
			if err != nil || len(parts) != 2 || parts[0] != "/tmp/executable with space" {
				t.Fatalf("command split: %v %v", parts, err)
			}
		}
	}
	for _, raw := range []string{
		`null`, `{"name":"x","command":"/bin/cat"}`, `{"type":"http","name":"x","url":"credential","headers":[]}`,
		`{"type":"http","name":"x","url":"http://127.0.0.1","headers":[{"name":"Authorization","value":"credential\r\nInjected: true"}]}`,
	} {
		_, err := ParseACPServers([]json.RawMessage{json.RawMessage(raw)}, "/tmp")
		if err == nil || strings.Contains(err.Error(), "credential") {
			t.Fatalf("invalid config/privacy: %v", err)
		}
	}
}
