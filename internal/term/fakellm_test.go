//go:build termtest

package term

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeLLM is a provider that speaks both protocols. The test decides what each
// request gets by setting Handler.
type fakeLLM struct {
	srv     *httptest.Server
	Handler func(q *llmReq)
	mu      sync.Mutex
	reqs    []string // request bodies
	Models  []string
}

// llmReq is one model request.
type llmReq struct {
	N     int    // 1-based index of the request
	Proto string // "chat" or "responses"
	Body  string // raw request body
	w     http.ResponseWriter
	r     *http.Request
	rel   chan struct{}
}

func newFakeLLM(t *testing.T) *fakeLLM {
	f := &fakeLLM{Models: []string{"fake-model", "other-model"}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeLLM) URL() string { return f.srv.URL }

// Requests returns how many model requests were made so far.
func (f *fakeLLM) Requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

// Bodies returns the raw bodies of all model requests.
func (f *fakeLLM) Bodies() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.reqs...)
}

func (f *fakeLLM) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/models") {
		var data []map[string]string
		for _, m := range f.Models {
			data = append(data, map[string]string{"id": m})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
		return
	}
	proto := "chat"
	if strings.HasSuffix(r.URL.Path, "/responses") {
		proto = "responses"
	}
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			break
		}
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, b.String())
	n := len(f.reqs)
	f.mu.Unlock()
	q := &llmReq{N: n, Proto: proto, Body: b.String(), w: w, r: r}
	if f.Handler == nil {
		q.Text("ok")
		return
	}
	f.Handler(q)
}

// Mentions reports whether the request body contains s (for example the output
// of a tool call that was fed back).
func (q *llmReq) Mentions(s string) bool { return strings.Contains(q.Body, s) }

func (q *llmReq) sse(event, data string) {
	if event != "" {
		fmt.Fprintf(q.w, "event: %s\n", event)
	}
	fmt.Fprintf(q.w, "data: %s\n\n", data)
	q.w.(http.Flusher).Flush()
}

func (q *llmReq) head() {
	q.w.Header().Set("Content-Type", "text/event-stream")
	q.w.WriteHeader(200)
	q.w.(http.Flusher).Flush()
}

func js(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Text streams the parts as one answer and finishes it.
func (q *llmReq) Text(parts ...string) {
	q.head()
	q.TextPartial(parts...)
	q.finish()
}

// TextPartial streams text without finishing the answer.
func (q *llmReq) TextPartial(parts ...string) {
	for _, p := range parts {
		if q.Proto == "chat" {
			q.sse("", js(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": p}}}}))
		} else {
			q.sse("", js(map[string]any{"type": "response.output_text.delta", "delta": p}))
		}
		time.Sleep(15 * time.Millisecond)
	}
}

func (q *llmReq) finish() {
	usage := map[string]int{"prompt_tokens": 100, "completion_tokens": 20}
	if q.Proto == "chat" {
		q.sse("", js(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{}, "finish_reason": "stop"}}}))
		q.sse("", js(map[string]any{"choices": []any{}, "usage": usage}))
		q.sse("", "[DONE]")
		return
	}
	q.sse("", js(map[string]any{"type": "response.completed", "response": map[string]any{"status": "completed", "usage": map[string]int{"input_tokens": 100, "output_tokens": 20}}}))
}

// Tool streams a call of the bash tool, after optional text.
func (q *llmReq) Tool(preface, command string) { q.Tools(preface, command) }

// Tools streams several bash calls in one response, after optional text.
func (q *llmReq) Tools(preface string, commands ...string) {
	q.head()
	if preface != "" {
		q.TextPartial(preface)
	}
	if q.Proto == "chat" {
		var calls []any
		for i, command := range commands {
			args := js(map[string]string{"command": command, "title": "Inspect shell output"})
			calls = append(calls, map[string]any{"index": i, "id": fmt.Sprintf("call_%d_%d", q.N, i), "function": map[string]any{"name": "bash", "arguments": args}})
		}
		q.sse("", js(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": calls}}}}))
		q.sse("", js(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{}, "finish_reason": "tool_calls"}}}))
		q.sse("", js(map[string]any{"choices": []any{}, "usage": map[string]int{"prompt_tokens": 100, "completion_tokens": 20}}))
		q.sse("", "[DONE]")
		return
	}
	for i, command := range commands {
		args := js(map[string]string{"command": command, "title": "Inspect shell output"})
		id := fmt.Sprintf("call_%d_%d", q.N, i)
		item := map[string]any{"type": "function_call", "id": "fc_" + id, "call_id": id, "name": "bash", "arguments": args}
		q.sse("", js(map[string]any{"type": "response.output_item.added", "item": map[string]any{"type": "function_call", "id": "fc_" + id, "call_id": id, "name": "bash"}}))
		q.sse("", js(map[string]any{"type": "response.function_call_arguments.delta", "item_id": "fc_" + id, "delta": args}))
		q.sse("", js(map[string]any{"type": "response.output_item.done", "item": item}))
	}
	q.finish()
}

// Fail answers with an HTTP error (retryAfter may be empty).
func (q *llmReq) Fail(status int, retryAfter string) {
	if retryAfter != "" {
		q.w.Header().Set("Retry-After", retryAfter)
	}
	http.Error(q.w, fmt.Sprintf("fake failure %d", status), status)
}

// Drop streams some text and then cuts the connection.
func (q *llmReq) Drop(partial string) {
	q.head()
	q.TextPartial(partial)
	conn, _, err := q.w.(http.Hijacker).Hijack()
	if err == nil {
		conn.Close()
	}
}

// Hang streams some text and then waits until the client goes away.
func (q *llmReq) Hang(partial string) {
	q.head()
	q.TextPartial(partial)
	<-q.r.Context().Done()
}
