package acp

import (
	"context"
	"encoding/json"
	"meno/internal/api"
	"meno/internal/client"
	"strings"
	"testing"
	"time"
)

func TestInitializeProtocolContract(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"protocolVersion":"1"}`, `{"protocolVersion":true}`} {
		t.Run(raw, func(t *testing.T) {
			s := &Server{}
			if _, err := s.initialize(json.RawMessage(raw)); err == nil {
				t.Fatal("malformed initialize accepted")
			}
		})
	}
	// An unsupported or absent version is negotiated down to the supported one.
	for _, raw := range []string{`{}`, `{"protocolVersion":0}`, `{"protocolVersion":1}`, `{"protocolVersion":2}`, `{"protocolVersion":65535}`, `{"protocolVersion":99999999999999999999}`, `{"protocolVersion":-1}`, `{"protocolVersion":null}`} {
		t.Run("negotiate"+raw, func(t *testing.T) {
			v, err := (&Server{}).initialize(json.RawMessage(raw))
			if err != nil {
				t.Fatal(err)
			}
			if got := v.(map[string]any)["protocolVersion"]; got != ProtocolVersion {
				t.Fatalf("protocolVersion = %v", got)
			}
		})
	}
	for _, caps := range []string{``, `,"clientCapabilities":{}`, `,"clientCapabilities":{"terminal":true,"fs":{"readTextFile":true}}`, `,"clientCapabilities":"invalid"`} {
		s := &Server{}
		v, err := s.initialize(json.RawMessage(`{"protocolVersion":1` + caps + `}`))
		if err != nil {
			t.Fatal(err)
		}
		wire := string(mustJSON(v))
		if !strings.Contains(wire, `"http":true`) || !strings.Contains(wire, `"sse":true`) {
			t.Fatalf("MCP capabilities: %s", wire)
		}
		for _, absent := range []string{`"resume"`, `"fork"`, `"modes"`, `"terminal"`} {
			if strings.Contains(wire, absent) {
				t.Errorf("unimplemented capability: %s", absent)
			}
		}
	}
}

func TestRPCInvalidEnvelopeAndNotifications(t *testing.T) {
	for _, raw := range []string{`{"jsonrpc":"1.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":{},"method":"initialize"}`, `[]`, `null`, `{"jsonrpc":"2.0","id":1}`} {
		if got := runRaw(t, raw+"\n"); !strings.Contains(got, `"code":-32600`) {
			t.Errorf("accepted invalid envelope %s: %s", raw, got)
		}
	}
	// A notification must not execute a request-only method (or panic on a nil backend).
	if got := runRaw(t, `{"jsonrpc":"2.0","method":"session/new","params":{"cwd":"/tmp"}}`+"\n"); got != "" {
		t.Fatalf("notification response: %s", got)
	}
}

func TestRPCInternalErrorsAreRedacted(t *testing.T) {
	var out lockedBuf
	c := newConn(&out)
	c.dispatch([]byte(`{"jsonrpc":"2.0","id":1,"method":"secret"}`), false, map[string]handler{"secret": func(json.RawMessage) (any, error) { panic("secret-token") }})
	c.wg.Wait()
	if strings.Contains(out.String(), "secret-token") || !strings.Contains(out.String(), `"code":-32603`) {
		t.Fatalf("unsafe error: %s", out.String())
	}
}

func TestDiagnosticsOnlyMetadata(t *testing.T) {
	var out lockedBuf
	d := &diagnostics{out: &out}
	d.record("secret-prompt-token", codeInternal, time.Now())
	if strings.Contains(out.String(), "secret") || !strings.Contains(out.String(), `"method":"unknown"`) {
		t.Fatal(out.String())
	}
	if got := runRaw(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}`+"\n"); strings.Contains(got, "duration_ms") {
		t.Fatal("diagnostics polluted stdout")
	}
}

func TestMCPSetupErrorsStayActionableWithoutSecrets(t *testing.T) {
	b := &resourceErrorBackend{}
	s := &Server{backend: b, ctx: context.Background()}
	_, err := s.configureMCP("session", "/tmp", []json.RawMessage{json.RawMessage(`{"type":"http","name":"tools","url":"http://127.0.0.1","headers":[]}`)}, nil)
	if err == nil || !strings.Contains(err.Error(), "MCP") || strings.Contains(err.Error(), "private-secret") {
		t.Fatalf("setup error: %v", err)
	}
}

type resourceErrorBackend struct{ Backend }

func (*resourceErrorBackend) Resource(context.Context, string, string, string, api.ResourceRequest) (api.ResourceResponse, error) {
	return api.ResourceResponse{}, &client.RemoteError{Code: api.CodeConflict, Message: "private-secret"}
}
