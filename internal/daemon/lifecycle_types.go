package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Options configures a Server. The server has no process lifecycle of its own:
// the process that owns the engine decides when it starts and stops.
type Options struct {
	Version string
	OnStart func() error
	OnStop  func()
	Backend Backend
	Hub     *Hub
	Global  *GlobalHub
}

// Server routes the HTTP/SSE protocol to a Backend.
type Server struct {
	version, bootID string
	pid             int
	started         time.Time
	backend         Backend
	hub             *Hub
	global          *GlobalHub
	onStart         func() error
	onStop          func()
	stopOnce        sync.Once
	mu              sync.Mutex
	closers         []io.Closer
	servers         map[*http.Server]struct{}
	handler         http.Handler
}

// New creates a server. Serve connections with Serve or ServeConn.
func New(o Options) *Server {
	if o.Backend == nil {
		o.Backend = NotImplementedBackend{}
	}
	if o.Version == "" {
		o.Version = "dev"
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	boot := hex.EncodeToString(b)
	hub := o.Hub
	if hub == nil {
		hub = NewHub(boot, 2000)
	}
	global := o.Global
	if global == nil {
		global = NewGlobalHub(boot, 2000)
	}
	hub.SetBootID(boot)
	global.SetBootID(boot)
	s := &Server{version: o.Version, bootID: boot, pid: os.Getpid(), started: time.Now(), backend: o.Backend, hub: hub, global: global, onStart: o.OnStart, onStop: o.OnStop, servers: map[*http.Server]struct{}{}}
	s.handler = s.routes()
	return s
}

// BootID returns the server boot identifier.
func (s *Server) BootID() string { return s.bootID }
