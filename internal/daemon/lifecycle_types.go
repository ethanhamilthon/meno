package daemon

import (
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

// Options configures daemon lifecycle and transport. IdleExit accepts a duration;
// IdleExitMin accepts the corresponding setting in minutes.
type Options struct {
	Home, Version string
	IdleExit      time.Duration
	IdleExitMin   int
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
	http                                           *http.Server
	listener                                       net.Listener
	stopOnce                                       sync.Once
	done                                           chan struct{}
	lock                                           *os.File
}

// New creates a daemon. IdleExit sets a duration. IdleExitMin sets the matching setting in minutes.
// Zero disables idle exit.
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
	if idle == 0 && o.IdleExitMin > 0 {
		idle = time.Duration(o.IdleExitMin) * time.Minute
	}
	return &Server{home: o.Home, runDir: run, socket: SocketPath(o.Home), logPath: filepath.Join(logs, "daemon.log"), version: o.Version, bootID: boot, pid: os.Getpid(), started: time.Now(), idle: idle, backend: o.Backend, hub: hub, global: global, done: make(chan struct{})}, nil
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

// LogPath returns the daemon log path.
func (s *Server) LogPath() string { return s.logPath }
