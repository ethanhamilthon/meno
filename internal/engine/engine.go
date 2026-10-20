// Package engine owns daemon-side sessions and their agent runs.
package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/modes"
	"meno/internal/proc"
	"meno/internal/proto"
	"meno/internal/routing"
	"meno/internal/tools"
	"os"
	"sync"
	"time"
)

// ResourceHandler owns config resources outside the engine's session scope.
type ResourceHandler interface {
	// Resource serves the config routes (settings, providers, login, ...).
	Resource(context.Context, string, string, string, proto.ResourceRequest) (proto.ResourceResponse, error)
}

// EventHub publishes ordered daemon session events.
type EventHub interface {
	Publish(string, string, any) proto.Event
	Sequence(string) uint64
}

// GlobalEventHub publishes status across daemon sessions.
type GlobalEventHub interface {
	Publish(string, string, any) proto.Event
}

// Options configures the daemon engine. ProviderClient is a test seam.
type Options struct {
	Home           string
	EventHub       EventHub
	GlobalHub      GlobalEventHub
	Resources      ResourceHandler
	ProviderClient func(llm.Provider) llm.Client
	// Socket is the path of the daemon socket (MENO_SOCK of agent calls).
	Socket string
}

type liveCall struct {
	id, code, title, output string
	limit, elapsed          time.Duration
	exit                    int
	timedOut                bool
}
type runState struct {
	session          *agent.Session
	cwd, prevCwd     string
	env              map[string]string
	running          bool
	phase, requestID string
	cancel           context.CancelFunc
	queue            agent.Queue
	live             proto.LiveState
	ask              *askState
	retry            *proto.RetryState
	metrics          proto.Metrics
	goal             *agent.Goal
	modes            modes.Combined
	token            tokenState
	changed          time.Time
	prompt           promptState
	sticky           *routing.Sticky
	lastRequests     []llm.Request
	lastRequest      llm.Request
	lastErr          string
	spawned          int
	pinProvider      string // provider, model and effort the running request started with
	pinModel         string
	pinEffort        string
	cancelled        map[string]bool        // root requests that were stopped; their events never wake the session
	block            string                 // a child asked to be blocked in the current tool batch
	calls            map[string]*tools.Call // running bash calls by id (ask pauses their timer)
	published        int
	mu               sync.Mutex
}

// Engine owns loaded sessions, agent loops, tool services and event translation.
// Construct it with New and pass it to daemon.New as its Backend.
type Engine struct {
	events2   *agent.EventStore // durable steering queue
	home      string
	sock      string // the daemon socket, handed to agent calls as MENO_SOCK
	paths     config.Paths
	store     *config.Store
	settings  config.Settings
	providers []llm.Provider
	servers   []mcp.Server
	catalog   map[string]llm.ModelInfo
	mcp       *mcp.Manager
	tasks     *proc.Supervisor
	children  *agent.ChildStore
	events    EventHub
	global    GlobalEventHub
	mcpCtx    context.Context
	mcpStop   context.CancelFunc
	resources ResourceHandler
	client    func(llm.Provider) llm.Client
	mu        sync.Mutex
	sessions  map[string]*runState
	ids       map[string]idResult
	closed    bool
	runs      sync.WaitGroup
}
type idResult struct {
	value any
	err   error
}

// New opens shared SQLite-backed services and recovers interrupted work.
// New opens daemon-owned services, performs crash recovery, and returns a Backend.
func New(o Options) (*Engine, error) {
	if o.Home == "" {
		return nil, errors.New("engine home is required")
	}
	paths := config.Paths{Home: o.Home}
	st := config.NewStore(paths)
	settings, err := st.LoadSettings()
	if err != nil {
		return nil, err
	}
	providers, err := st.LoadProviders()
	if err != nil {
		return nil, err
	}
	files, _ := config.LoadProviderFile(paths.ProviderFile(), os.Getenv)
	providers, _ = config.MergeProviders(providers, files)
	servers, err := st.LoadServers()
	if err != nil {
		return nil, err
	}
	serverFiles, _ := config.LoadMCPFile(paths.MCPFile())
	servers, _ = config.MergeServers(servers, serverFiles)
	catalog, err := st.LoadCatalog()
	if err != nil {
		return nil, err
	}
	d, err := db.Open(o.Home)
	if err != nil {
		return nil, err
	}
	eng := &Engine{}
	tasks, err := proc.New(d, paths.File("task-logs"), proc.Hooks{Finished: func(t proc.Task) { eng.onTaskFinished(t) }})
	if err != nil {
		return nil, err
	}
	evStore, _ := agent.OpenEvents(o.Home)
	children, err := agent.OpenChildren(o.Home)
	if err != nil {
		return nil, err
	}
	if err = children.MarkInterrupted(); err != nil {
		tasks.Shutdown()
		return nil, err
	}
	if err = recoverInterrupted(o.Home, d); err != nil {
		tasks.Shutdown()
		return nil, err
	}
	mcpManager := mcp.NewManager()
	client := o.ProviderClient
	if client == nil {
		client = llm.NewClient
	}
	mcpCtx, mcpStop := context.WithCancel(context.Background())
	for _, server := range servers {
		if server.Enabled {
			server := server
			go func() { _ = mcpManager.Start(mcpCtx, server) }()
		}
	}
	*eng = Engine{events2: evStore, sock: o.Socket, home: o.Home, paths: paths, store: st, settings: settings, providers: providers, servers: servers, catalog: catalog, mcp: mcpManager, tasks: tasks, children: children, events: o.EventHub, global: o.GlobalHub, resources: o.Resources, client: client, mcpCtx: mcpCtx, mcpStop: mcpStop, sessions: map[string]*runState{}, ids: map[string]idResult{}}
	return eng, nil
}

// Close stops owned child/task/MCP services but does not react to client disconnects.
// Close cancels and joins active runs and stops tasks and MCP processes.
func (e *Engine) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	all := make([]*runState, 0, len(e.sessions))
	for _, s := range e.sessions {
		all = append(all, s)
	}
	e.mu.Unlock()
	for _, s := range all {
		s.mu.Lock()
		if s.cancel != nil {
			s.cancel()
		}
		s.mu.Unlock()
	}
	e.runs.Wait()
	e.mcpStop()
	e.tasks.Shutdown()
	e.mcp.StopAll()
}

func (e *Engine) publish(id, typ string, data any) {
	if e.events != nil {
		e.events.Publish(id, typ, data)
	}
	if e.global != nil {
		e.global.Publish(id, proto.EventGlobalStatus, proto.GlobalStatus{SessionID: id, Running: e.running(id), NeedsYou: e.needsYou(id), DoneUnseen: typ == proto.EventDone})
	}
}
func (e *Engine) running(id string) bool {
	e.mu.Lock()
	s := e.sessions[id]
	e.mu.Unlock()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
func (e *Engine) needsYou(id string) bool {
	e.mu.Lock()
	s := e.sessions[id]
	e.mu.Unlock()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ask != nil
}
func (e *Engine) save(s *runState) error { return agent.SaveSession(e.home, s.session) }
func (e *Engine) load(ctx context.Context, id string) (*runState, error) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, errors.New("engine is closed")
	}
	if s := e.sessions[id]; s != nil {
		e.mu.Unlock()
		return s, nil
	}
	e.mu.Unlock()
	sess, err := agent.LoadSession(e.home, id)
	if err != nil {
		return nil, err
	}
	sess.AttachDir = e.paths.File("attachments")
	tok, err := newToken()
	if err != nil {
		return nil, err
	}
	st := &runState{cancelled: map[string]bool{}, calls: map[string]*tools.Call{}, session: sess, cwd: sess.Cwd, env: map[string]string{}, changed: time.Now(), token: tokenState{value: tok, groups: groupsFor(sess.Modes)}}
	st.modes = compose(sess.Modes)
	st.published = len(sess.Snapshot())
	e.mu.Lock()
	if old := e.sessions[id]; old != nil {
		e.mu.Unlock()
		return old, nil
	}
	e.sessions[id] = st
	e.mu.Unlock()
	return st, nil
}
func newToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func groupsFor(names []string) map[string]bool {
	m := map[string]bool{}
	for _, g := range []string{"tasks", "agents", "mcp", "web", "view", "goal", "models", "ask", "help"} {
		m[g] = true
	}
	for _, n := range names {
		if n == "subagents" {
			m = map[string]bool{"agents": true, "ask": true, "goal": true, "help": true}
			break
		}
	}
	return m
}
