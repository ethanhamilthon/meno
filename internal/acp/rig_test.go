package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/configsvc"
	"meno/internal/daemon"
	"meno/internal/db"
	"meno/internal/engine"
	"meno/internal/llm"
	"meno/internal/mcp"
)

// daemonRig starts a real daemon and engine with a fake provider.
func daemonRig(t *testing.T, f llm.Client) (*client.Client, *engine.Engine) {
	t.Helper()
	home := t.TempDir()
	st := config.NewStore(config.Paths{Home: home})
	s := config.DefaultSettings()
	s.Provider, s.Model = "test", "fake"
	if err := st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	p := llm.Provider{Name: "test", BaseURL: "http://127.0.0.1", Protocol: llm.ProtoResponses, Models: map[string]llm.ModelSettings{"fake": {Context: 100000}}}
	if err := st.SaveProviders([]llm.Provider{p}); err != nil {
		t.Fatal(err)
	}
	hub, global := daemon.NewHub("b", 2000), daemon.NewGlobalHub("b", 100)
	manager := mcp.NewManager()
	svc, err := configsvc.New(configsvc.Options{Paths: config.Paths{Home: home}, MCP: manager,
		FetchMetadata: func(context.Context) (map[string]llm.ModelInfo, error) { return nil, nil },
		FetchModels: func(context.Context, llm.Provider) ([]llm.Model, error) {
			return []llm.Model{{ID: "fake", Provider: "test"}}, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Options{Resources: svc, MCP: manager, Home: home, EventHub: hub, GlobalHub: global, ProviderClient: func(llm.Provider) llm.Client { return f }})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := daemon.New(daemon.Options{Home: home, Backend: eng, Hub: hub, Global: global, Version: "test", OnStart: svc.Start, OnStop: func() { eng.Close(); svc.Close() }})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan struct{})
	go func() { _ = srv.Serve(ctx); close(served) }()
	t.Cleanup(func() {
		cancel()
		srv.Stop("test")
		<-served
		eng.Close()
		db.CloseAll()
	})
	c := client.New(home)
	for i := 0; ; i++ {
		if _, err := c.Hello(context.Background()); err == nil {
			break
		}
		if i > 400 {
			t.Fatal("daemon did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return c, eng
}

// peer is an ACP client driving Run over pipes.
type peer struct {
	t      *testing.T
	in     io.WriteCloser
	mu     sync.Mutex
	notes  []message
	waits  map[string]chan message
	nextID int
	done   chan error
}

func startPeer(t *testing.T, b Backend) *peer {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	p := &peer{t: t, in: inW, waits: map[string]chan message{}, done: make(chan error, 1)}
	go func() {
		p.done <- Run(context.Background(), inR, outW, b, Options{Version: "test"})
		_ = outW.Close()
	}()
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 1<<20), 32<<20)
		for sc.Scan() {
			var m message
			if json.Unmarshal(sc.Bytes(), &m) != nil {
				continue
			}
			p.mu.Lock()
			if len(m.ID) > 0 && m.Method == "" {
				if ch := p.waits[string(m.ID)]; ch != nil {
					ch <- m
				}
			} else {
				p.notes = append(p.notes, m)
			}
			p.mu.Unlock()
		}
	}()
	t.Cleanup(func() { _ = inW.Close() })
	return p
}

func (p *peer) sendRaw(line string) { _, _ = io.WriteString(p.in, line+"\n") }

// start sends a request and returns the channel of its response.
func (p *peer) start(method string, params any) chan message {
	p.mu.Lock()
	p.nextID++
	id := p.nextID
	ch := make(chan message, 1)
	idRaw := mustJSON(id)
	p.waits[string(idRaw)] = ch
	p.mu.Unlock()
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	p.sendRaw(string(raw))
	return ch
}

func (p *peer) call(method string, params any) message {
	p.t.Helper()
	select {
	case m := <-p.start(method, params):
		return m
	case <-time.After(15 * time.Second):
		p.t.Fatalf("%s timed out", method)
		return message{}
	}
}

func (p *peer) notify(method string, params any) {
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	p.sendRaw(string(raw))
}

// updates returns the session/update payloads of one kind.
func (p *peer) updates(kind string) []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []map[string]any
	for _, m := range p.notes {
		if m.Method != "session/update" {
			continue
		}
		var v struct {
			Update map[string]any `json:"update"`
		}
		if json.Unmarshal(m.Params, &v) == nil && v.Update["sessionUpdate"] == kind {
			out = append(out, v.Update)
		}
	}
	return out
}

func (p *peer) text(kind string) string {
	s := ""
	for _, u := range p.updates(kind) {
		if c, ok := u["content"].(map[string]any); ok {
			s += c["text"].(string)
		}
	}
	return s
}

func (p *peer) waitFor(what string, ok func() bool) {
	p.t.Helper()
	for i := 0; i < 600; i++ {
		if ok() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	p.t.Fatalf("timed out waiting for %s", what)
}

func result(t *testing.T, m message) map[string]any {
	t.Helper()
	if m.Error != nil {
		t.Fatalf("rpc error: %+v", m.Error)
	}
	var v map[string]any
	_ = json.Unmarshal(m.Result, &v)
	return v
}
