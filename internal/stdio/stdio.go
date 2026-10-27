// Package stdio serves meno's own JSON-RPC 2.0 protocol over a byte stream
// (meno -p --stdio). Every method is one typed call of the engine client, so
// validation and errors stay where they are; events arrive as notifications.
// ACP, for editors, is a separate adapter (internal/acp).
package stdio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"sync"

	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/jsonrpc"
)

// Options configures a server.
type Options struct {
	// NoAsk creates sessions without the ask tool unless the request says otherwise.
	NoAsk bool
}

// Server is one JSON-RPC peer talking to the engine.
type Server struct {
	c    *client.Client
	opts Options
	conn *jsonrpc.Conn
	ctx  context.Context

	mu   sync.Mutex
	subs map[string]context.CancelFunc
	wg   sync.WaitGroup
}

// Run serves in/out until EOF on in or ctx ends, then stops all subscriptions.
func Run(ctx context.Context, in io.Reader, out io.Writer, c *client.Client, o Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := &Server{c: c, opts: o, conn: jsonrpc.NewConn(out), ctx: ctx, subs: map[string]context.CancelFunc{}}
	err := s.conn.Serve(in, s.methods(), cancel)
	cancel()
	s.wg.Wait()
	return err
}

// call adapts a typed function to a handler.
func call[Q, R any](s *Server, f func(ctx context.Context, q Q) (R, error)) jsonrpc.Handler {
	return func(raw json.RawMessage) (any, error) {
		var q Q
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &q); err != nil {
				return nil, jsonrpc.ErrParams("invalid params: %v", err)
			}
		}
		r, err := f(s.ctx, q)
		return r, convert(err)
	}
}

// onSession adapts a function of a session: params carry "id" next to the request fields.
func onSession[Q, R any](s *Server, f func(ctx context.Context, id string, q Q) (R, error)) jsonrpc.Handler {
	return func(raw json.RawMessage) (any, error) {
		var head struct {
			ID string `json:"id"`
		}
		var q Q
		if err := json.Unmarshal(raw, &head); err != nil || head.ID == "" {
			return nil, jsonrpc.ErrParams("params need a session id")
		}
		if err := json.Unmarshal(raw, &q); err != nil {
			return nil, jsonrpc.ErrParams("invalid params: %v", err)
		}
		r, err := f(s.ctx, head.ID, q)
		return r, convert(err)
	}
}

// convert keeps the code of an engine error so a client can act on it.
func convert(err error) error {
	if err == nil {
		return nil
	}
	var remote *client.RemoteError
	if errors.As(err, &remote) {
		data, _ := json.Marshal(map[string]string{"code": remote.Code})
		return &jsonrpc.Error{Code: jsonrpc.CodeInternal, Message: remote.Message, Data: data}
	}
	return &jsonrpc.Error{Code: jsonrpc.CodeInternal, Message: err.Error()}
}

type empty struct{}

func (s *Server) methods() map[string]jsonrpc.Handler {
	c := s.c
	m := map[string]jsonrpc.Handler{
		"hello":        call(s, func(ctx context.Context, _ empty) (api.Hello, error) { return c.Hello(ctx) }),
		"session.list": call(s, c.ListSessions),
		"session.create": call(s, func(ctx context.Context, q api.SessionCreateRequest) (api.SessionCreateResponse, error) {
			if q.ClientID == "" {
				q.ClientID = newID()
			}
			q.NoAsk = q.NoAsk || s.opts.NoAsk
			return c.CreateSession(ctx, q)
		}),
		"session.get":   onSession(s, func(ctx context.Context, id string, _ empty) (api.Snapshot, error) { return c.Snapshot(ctx, id) }),
		"session.patch": onSession(s, c.PatchSession),
		"session.delete": onSession(s, func(ctx context.Context, id string, _ empty) (api.Ack, error) {
			return api.Ack{OK: true}, c.DeleteSession(ctx, id)
		}),
		"session.fork": onSession(s, func(ctx context.Context, id string, q api.ForkRequest) (api.ForkResponse, error) {
			if q.ClientID == "" {
				q.ClientID = newID()
			}
			return c.Fork(ctx, id, q)
		}),
		"session.turns": onSession(s, c.Turns),
		"session.message": onSession(s, func(ctx context.Context, id string, q api.MessageRequest) (api.MessageResponse, error) {
			if q.ClientID == "" {
				q.ClientID = newID()
			}
			if q.Client == "" {
				q.Client = "stdio"
			}
			return c.Message(ctx, id, q)
		}),
		"session.stop": onSession(s, func(ctx context.Context, id string, q api.StopRequest) (api.Ack, error) {
			return api.Ack{OK: true}, c.Stop(ctx, id, q)
		}),
		"session.compact": onSession(s, c.Compact),
		"session.answer": onSession(s, func(ctx context.Context, id string, q struct {
			AskID string `json:"ask_id"`
			api.AskAnswerRequest
		}) (api.Ack, error) {
			return api.Ack{OK: true}, c.AnswerAsk(ctx, id, q.AskID, q.AskAnswerRequest)
		}),
		"session.resource": onSession(s, s.resource),
		"subscribe":        s.subscribe,
		"unsubscribe":      s.unsubscribe,
		"config.get":       call(s, func(ctx context.Context, _ empty) (api.ConfigBundle, error) { return c.Config(ctx) }),
		"settings.get":     call(s, func(ctx context.Context, _ empty) (api.SettingsResponse, error) { return c.Settings(ctx) }),
		"settings.patch": call(s, func(ctx context.Context, q struct {
			Fields map[string]any `json:"fields"`
			Expect map[string]any `json:"expect,omitempty"`
		}) (api.SettingsResponse, error) {
			return c.PatchSettings(ctx, q.Fields, q.Expect)
		}),
		"providers.list":   call(s, func(ctx context.Context, _ empty) (api.ProvidersResponse, error) { return c.Providers(ctx) }),
		"providers.create": call(s, c.CreateProvider),
		"providers.update": call(s, func(ctx context.Context, q struct {
			Name string `json:"name"`
			api.ConfigProvider
		}) (api.ProviderResponse, error) {
			return c.UpdateProvider(ctx, q.Name, q.ConfigProvider)
		}),
		"providers.delete": call(s, func(ctx context.Context, q struct {
			Name string `json:"name"`
		}) (api.Ack, error) {
			return api.Ack{OK: true}, c.DeleteProvider(ctx, q.Name)
		}),
		"providers.enable": call(s, func(ctx context.Context, q struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}) (api.ProviderResponse, error) {
			return c.SetProviderEnabled(ctx, q.Name, q.Enabled)
		}),
		"skills.list": call(s, func(ctx context.Context, q struct {
			Cwd string `json:"cwd"`
		}) (api.SkillsResponse, error) {
			return c.Skills(ctx, q.Cwd)
		}),
		"skills.enable": call(s, func(ctx context.Context, q struct {
			Name string `json:"name"`
			api.SkillEnabledRequest
		}) (api.ConfigSkill, error) {
			return c.SetSkillEnabled(ctx, q.Name, q.SkillEnabledRequest)
		}),
		"mcp.list":    call(s, func(ctx context.Context, _ empty) (api.McpServersResponse, error) { return c.McpServers(ctx) }),
		"mcp.add":     call(s, c.AddMcpServer),
		"mcp.remove":  s.byName(c.RemoveMcpServer),
		"mcp.restart": s.byName(c.RestartMcpServer),
		"models.list": call(s, c.Models),
		"fs.search":   call(s, c.SearchFiles),
	}
	for _, t := range []string{"list", "status", "logs", "wait", "cancel"} {
		m["task."+t] = s.task(t)
	}
	return m
}

func (s *Server) byName(f func(context.Context, string) error) jsonrpc.Handler {
	return call(s, func(ctx context.Context, q struct {
		Name string `json:"name"`
	}) (api.Ack, error) {
		return api.Ack{OK: true}, f(ctx, q.Name)
	})
}

// resource calls a session route of the engine without an agent token.
func (s *Server) resource(ctx context.Context, id string, q struct {
	Method string          `json:"method"`
	Group  string          `json:"group"`
	Op     string          `json:"op,omitempty"`
	Query  string          `json:"query,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}) (api.ResourceResponse, error) {
	if q.Method == "" {
		q.Method = "POST"
	}
	p := "/v1/sessions/" + url.PathEscape(id) + "/" + url.PathEscape(q.Group)
	if q.Op != "" {
		p += "/" + url.PathEscape(q.Op)
	}
	return s.c.Resource(ctx, q.Method, p, q.Query, api.ResourceRequest{Data: q.Data})
}

// task maps task.<op> to the tasks route of a session. Params are the route's
// own (for example {"id": TASK_ID}) plus "session", the session that owns the task.
func (s *Server) task(op string) jsonrpc.Handler {
	return func(raw json.RawMessage) (any, error) {
		rest := map[string]json.RawMessage{}
		if err := json.Unmarshal(raw, &rest); err != nil {
			return nil, jsonrpc.ErrParams("invalid params: %v", err)
		}
		var session string
		if json.Unmarshal(rest["session"], &session) != nil || session == "" {
			return nil, jsonrpc.ErrParams("params need a session")
		}
		delete(rest, "session")
		data, _ := json.Marshal(rest)
		method := "POST"
		if op == "list" {
			method = "GET"
		}
		r, err := s.c.Resource(s.ctx, method, "/v1/sessions/"+url.PathEscape(session)+"/tasks/"+op, "", api.ResourceRequest{Data: data})
		return r, convert(err)
	}
}

func (s *Server) subscribe(raw json.RawMessage) (any, error) {
	var q struct {
		Session  string `json:"session,omitempty"` // empty: the global stream
		AfterSeq uint64 `json:"after_seq,omitempty"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &q); err != nil {
			return nil, jsonrpc.ErrParams("invalid params: %v", err)
		}
	}
	id := newID()
	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.subs[id] = cancel
	s.mu.Unlock()
	send := func(e api.Event) error {
		return s.conn.Notify("event", map[string]any{"subscription": id, "event": e})
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.drop(id)
		if q.Session == "" {
			_ = s.c.WatchGlobal(ctx, send)
		} else {
			_ = s.c.Stream(ctx, q.Session, q.AfterSeq, "", send)
		}
	}()
	return map[string]string{"subscription": id}, nil
}

func (s *Server) unsubscribe(raw json.RawMessage) (any, error) {
	var q struct {
		Subscription string `json:"subscription"`
	}
	if err := json.Unmarshal(raw, &q); err != nil || q.Subscription == "" {
		return nil, jsonrpc.ErrParams("params need a subscription")
	}
	s.drop(q.Subscription)
	return api.Ack{OK: true}, nil
}

func (s *Server) drop(id string) {
	s.mu.Lock()
	cancel := s.subs[id]
	delete(s.subs, id)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func newID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
