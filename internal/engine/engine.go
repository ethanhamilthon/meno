// Package engine owns daemon-side sessions and their agent runs.
package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/process"
	"os"
	"sync"
	"time"
)

// ResourceHandler owns config resources outside the engine's session scope.
type ResourceHandler interface {
	// Resource serves the config routes (settings, providers, login, ...).
	Resource(context.Context, string, string, string, api.ResourceRequest) (api.ResourceResponse, error)
}

// EventHub publishes ordered daemon session events.
type EventHub interface {
	Publish(string, string, any) api.Event
	Sequence(string) uint64
}

// GlobalEventHub publishes status across daemon sessions.
type GlobalEventHub interface {
	Publish(string, string, any) api.Event
}

// Options configures the daemon engine. ProviderClient is a test seam.
type Options struct {
	MCP            *mcp.Manager
	Home           string
	EventHub       EventHub
	GlobalHub      GlobalEventHub
	Resources      ResourceHandler
	ProviderClient func(llm.Provider) llm.Client
	// Socket is the path of the daemon socket (MENO_SOCK of agent calls).
	Socket string
}

type runState struct {
	session          *agent.Session
	cwd, prevCwd     string
	env              map[string]string
	running          bool
	phase, requestID string
	cancel           context.CancelFunc
	queue            agent.Queue
	live             api.LiveState
	ask              *askState
	retry            *api.RetryState
	metrics          api.Metrics
	token            tokenState
	changed          time.Time
	prompt           promptState
	lastRequests     []llm.Request
	lastRequest      llm.Request
	lastErr          string
	pinProvider      string // provider, model and effort the running request started with
	pinModel         string
	pinEffort        string
	compacting       bool            // a compaction holds the history; a second one is rejected
	cancelled        map[string]bool // root requests that were stopped; their events never wake the session
	published        int
	visibleTurns     []api.Turn // published history, never runner-owned mutable state
	visibleQueue     []string
	origin           string // window that started the running turn (advisory)
	persisted        bool   // the session is in the database; a new one is only in memory until its first message
	// commit serializes a state change with the publication of its events, so a
	// snapshot cursor always matches the state it was read with. Lock order:
	// commit before mu; never hold commit across provider or socket I/O.
	commit sync.Mutex
	mu     sync.Mutex
}

// Engine owns loaded sessions, agent loops, tool services and event translation.
// Construct it with New and pass it to daemon.New as its Backend.
type Engine struct {
	events2    *agent.EventStore // durable steering queue
	home       string
	sock       string // the daemon socket, handed to agent calls as MENO_SOCK
	paths      config.Paths
	store      *config.Store
	settings   config.Settings
	providers  []llm.Provider
	servers    []mcp.Server
	catalog    map[string]llm.ModelInfo
	mcp        *mcp.Manager
	tasks      *process.Supervisor
	events     EventHub
	global     GlobalEventHub
	resources  ResourceHandler
	client     func(llm.Provider) llm.Client
	mu         sync.Mutex
	sessions   map[string]*runState
	ids        map[string]idResult
	closed     bool
	runs       sync.WaitGroup
	activeRuns int
}
type idResult struct {
	value any
	err   error
}

// New opens shared SQLite-backed services and recovers interrupted work.
// New opens daemon-owned services, performs crash recovery, and returns a Backend.
func New(o Options) (*Engine, error) {
	if o.MCP == nil {
		return nil, errors.New("engine MCP manager is required")
	}
	if o.Home == "" {
		return nil, errors.New("engine home is required")
	}
	paths := config.Paths{Home: o.Home}
	st := config.NewStore(paths)
	settings, err := loadSettings(o.Resources, st)
	if err != nil {
		return nil, err
	}
	// A provider or MCP file the user broke by hand must not stop the daemon: the
	// lists stay empty and the config service reports the file.
	providers, _ := st.LoadProvidersResolved(os.Getenv)
	servers, _ := st.LoadServers()
	catalog, err := st.LoadCatalog()
	if err != nil {
		return nil, err
	}
	d, err := db.Open(o.Home)
	if err != nil {
		return nil, err
	}
	eng := &Engine{}
	tasks, err := process.New(d, paths.File("task-logs"), process.Hooks{Finished: func(t process.Task) { eng.onTaskFinished(t) }})
	if err != nil {
		return nil, err
	}
	evStore, _ := agent.OpenEvents(o.Home)
	if err = recoverInterrupted(o.Home, d); err != nil {
		tasks.Shutdown()
		return nil, err
	}
	client := o.ProviderClient
	if client == nil {
		client = llm.NewClient
	}
	*eng = Engine{events2: evStore, sock: o.Socket, home: o.Home, paths: paths, store: st, settings: settings, providers: providers, servers: servers, catalog: catalog, mcp: o.MCP, tasks: tasks, events: o.EventHub, global: o.GlobalHub, resources: o.Resources, client: client, sessions: map[string]*runState{}, ids: map[string]idResult{}}
	return eng, nil
}

// Close stops owned task and MCP services but does not react to client disconnects.
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
	e.tasks.Shutdown()
}

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
	st, err := e.newState(sess, true)
	if err != nil {
		return nil, err
	}
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
	for _, g := range []string{"tasks", "mcp", "view", "models", "ask", "help"} {
		m[g] = true
	}
	return m
}
