package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrAlreadyRunning means the per-home daemon lock is held.
var ErrAlreadyRunning = errors.New("daemon is already running")

// Options configures daemon lifecycle. A positive IdleExit overrides the fixed
// production grace interval for tests. Zero selects the production policy.
type Options struct {
	StartupLock   *os.File
	Home, Version string
	OnStart       func(context.Context) error
	OnStop        func()
	IdleExit      time.Duration
	Backend       Backend
	Hub           *Hub
	Global        *GlobalHub
}

// Server owns the daemon HTTP transport and lifecycle.
type Server struct {
	home, runDir, socket, logPath, version, bootID string
	pid                                            int
	started                                        time.Time
	idle                                           time.Duration
	backend                                        Backend
	hub                                            *Hub
	global                                         *GlobalHub
	onStop                                         func()
	http                                           *http.Server
	listener                                       net.Listener
	admission                                      sync.RWMutex
	startMu                                        sync.Mutex
	stopping                                       bool
	requests                                       int
	requestChanged                                 chan struct{}
	idleMu                                         sync.Mutex
	lastActive                                     time.Time
	stopOnce                                       sync.Once
	done                                           chan struct{}
	lock                                           *os.File
	onStart                                        func(context.Context) error
}

// New creates a daemon with automatic idle shutdown enabled.
func New(o Options) (*Server, error) {
	if o.Home == "" {
		return nil, errors.New("daemon home is required")
	}
	if o.Backend == nil {
		o.Backend = NotImplementedBackend{}
	}
	if o.Version == "" {
		o.Version = "dev"
	}
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return nil, e
	}
	boot := hex.EncodeToString(b)
	run := filepath.Join(o.Home, "run")
	if e := privateDir(run); e != nil {
		return nil, e
	}
	logs := filepath.Join(o.Home, "log")
	if e := privateDir(logs); e != nil {
		return nil, e
	}
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
	idle := o.IdleExit
	if idle <= 0 {
		idle = idleExitDefault
	}
	return &Server{home: o.Home, runDir: run, socket: SocketPath(o.Home), logPath: filepath.Join(logs, "daemon.log"), version: o.Version, bootID: boot, pid: os.Getpid(), started: time.Now(), idle: idle, lastActive: time.Now(), requestChanged: make(chan struct{}), backend: o.Backend, hub: hub, global: global, done: make(chan struct{}), lock: o.StartupLock, onStart: o.OnStart, onStop: o.OnStop}, nil
}
func privateDir(path string) error {
	if e := os.MkdirAll(path, 0o700); e != nil {
		return e
	}
	return os.Chmod(path, 0o700)
}

// Socket returns the server unix socket path.
func (s *Server) Socket() string { return s.socket }

// BootID returns the server boot identifier.
func (s *Server) BootID() string { return s.bootID }

// GlobalHub returns the daemon-wide event hub for service notifications.
func (s *Server) GlobalHub() *GlobalHub { return s.global }
