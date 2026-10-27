package acp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"meno/internal/api"
)

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuf) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func runRaw(t *testing.T, input string) string {
	t.Helper()
	var out lockedBuf
	if err := Run(context.Background(), strings.NewReader(input), &out, nil, Options{Version: "t"}); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestMalformedAndUnknownInputKeepServing(t *testing.T) {
	out := runRaw(t, "not json\n\n"+
		`{"jsonrpc":"2.0","id":1,"method":"nope"}`+"\n"+
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":1}}`+"\n")
	if !strings.Contains(out, `"code":-32700`) || !strings.Contains(out, `"code":-32601`) || !strings.Contains(out, `"protocolVersion":1`) {
		t.Fatalf("output: %s", out)
	}
}

func TestOversizedMessageIsRejectedAndStreamContinues(t *testing.T) {
	big := strings.Repeat("x", MaxLineBytes+10)
	out := runRaw(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"pad":"`+big+`"}}`+"\n"+
		`{"jsonrpc":"2.0","id":2,"method":"initialize"}`+"\n")
	if !strings.Contains(out, "size limit") || !strings.Contains(out, `"id":2`) {
		t.Fatalf("output: %.200s", out)
	}
}

func TestFragmentedWritesAreReassembled(t *testing.T) {
	pr, pw := io.Pipe()
	var out lockedBuf
	done := make(chan struct{})
	go func() {
		_ = Run(context.Background(), pr, &out, nil, Options{})
		close(done)
	}()
	msg := `{"jsonrpc":"2.0","id":7,"method":"initialize"}` + "\n"
	for i := 0; i < len(msg); i += 5 {
		end := min(i+5, len(msg))
		_, _ = io.WriteString(pw, msg[i:end])
		time.Sleep(time.Millisecond)
	}
	_ = pw.Close()
	<-done
	if !strings.Contains(out.String(), `"id":7`) {
		t.Fatalf("output: %s", out.String())
	}
}

func TestDuplicateInFlightRequestIDIsRejected(t *testing.T) {
	var out lockedBuf
	c := newConn(&out)
	release := make(chan struct{})
	methods := map[string]handler{"slow": func(_ json.RawMessage) (any, error) { <-release; return nil, nil }}
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(release)
	}()
	_ = c.Serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"slow"}`+"\n"+`{"jsonrpc":"2.0","id":1,"method":"slow"}`+"\n"), methods, nil)
	if !strings.Contains(out.String(), "duplicate request id") {
		t.Fatalf("output: %s", out.String())
	}
}

func TestPromptContentIsValidatedNotDropped(t *testing.T) {
	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nxx"))
	in, err := parsePrompt([]block{{Type: "image", Data: png, MimeType: "image/png"}})
	if err != nil || len(in.Attachments) != 1 || in.Attachments[0].MIME != "image/png" || in.Attachments[0].Name != "image-1.png" {
		t.Fatalf("image-only prompt: %+v %v", in, err)
	}
	bad := []block{
		{Type: "image", Data: "***", MimeType: "image/png"},
		{Type: "image", Data: png, MimeType: "text/plain"},
		{Type: "audio", Data: "AA==", MimeType: "audio/wav"},
		{Type: "resource"},
		{Type: "text", Text: "  "},
	}
	for i, b := range bad {
		if _, err := parsePrompt([]block{b}); err == nil {
			t.Errorf("block %d accepted", i)
		}
	}
	txt := "package main"
	in, err = parsePrompt([]block{{Type: "text", Text: "see"}, {Type: "resource", Resource: &struct {
		URI      string  `json:"uri"`
		MimeType string  `json:"mimeType,omitempty"`
		Text     *string `json:"text,omitempty"`
		Blob     *string `json:"blob,omitempty"`
	}{URI: "file:///a.go", Text: &txt}}})
	if err != nil || !strings.Contains(in.Text, "file:///a.go") || !strings.Contains(in.Text, txt) {
		t.Fatalf("resource: %+v %v", in, err)
	}
}

func TestModelValueRoundTrip(t *testing.T) {
	for _, c := range [][2]string{{"openai", "gpt-5"}, {"a/b", "x:y/z"}, {"p", "m%2F"}} {
		p, m, ok := parseModelValue(modelValue(c[0], c[1]))
		if !ok || p != c[0] || m != c[1] {
			t.Errorf("%v -> %q %q %v", c, p, m, ok)
		}
	}
	if _, _, ok := parseModelValue("nothing"); ok {
		t.Fatal("accepted a value without a provider")
	}
}

func TestQuestionTextIsCopyable(t *testing.T) {
	txt := formatQuestion(api.AskForm{AskID: "a1", Questions: []api.AskQuestion{{ID: "q", Prompt: "Pick", Type: "single", Options: []string{"x", "y"}, Recommendation: "x"}}})
	for _, want := range []string{"question id: `a1`", "`x`", "/answer a1 {\"q\":\"<option id>\"}", "--cancel"} {
		if !strings.Contains(txt, want) {
			t.Errorf("missing %q in %s", want, txt)
		}
	}
}
