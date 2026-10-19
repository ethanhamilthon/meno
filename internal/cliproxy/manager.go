// Package cliproxy manages the cliproxyapi helper that turns OAuth logins
// (Claude, ChatGPT, Antigravity, xAI, Meta, Kimi) into a local OpenAI-style API.
// meno installs it under its own home, writes its config, and talks to its
// management API; the process itself is run by proc.Daemons.
package cliproxy

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"meno/internal/store"
	"meno/internal/update"
)

// Repo is the GitHub repository the helper is released from.
const Repo = "router-for-me/CLIProxyAPI"

// binaryName is the file inside the release archive.
const binaryName = "cli-proxy-api"

// State is what meno remembers about the helper. It holds the keys of the
// local server, so the file is private.
type State struct {
	Version       string    `json:"version"` // installed release tag in use
	Pending       string    `json:"pending"` // downloaded tag that starts with the next launch
	Port          int       `json:"port"`
	ClientKey     string    `json:"client_key"`
	ManagementKey string    `json:"management_key"`
	HasAccounts   bool      `json:"has_accounts"` // at least one enabled OAuth account
	CheckedAt     time.Time `json:"checked_at"`
	Latest        string    `json:"latest"`
}

// Manager owns the helper's files under dir and its state.
type Manager struct {
	Dir     string        // $MENO_HOME/cliproxy
	Release update.Client // where releases come from; tests replace it
	HTTP    *http.Client  // nil: a client with a 15 s timeout

	mu     sync.Mutex
	st     State
	loaded bool
}

// New returns the manager for the meno home directory home.
func New(home string) *Manager {
	return &Manager{Dir: filepath.Join(home, "cliproxy"), Release: update.Client{Repo: Repo}}
}

func (m *Manager) statePath() string  { return filepath.Join(m.Dir, "state.json") }
func (m *Manager) ConfigPath() string { return filepath.Join(m.Dir, "config.yaml") }
func (m *Manager) AuthDir() string    { return filepath.Join(m.Dir, "auth") }
func (m *Manager) LogDir() string     { return filepath.Join(m.Dir, "logs") }

// BinaryPath is where the release tag version keeps the executable.
func (m *Manager) BinaryPath(version string) string {
	return filepath.Join(m.Dir, "bin", version, binaryName)
}

// State returns a copy of the state, reading it on first use and creating the
// keys and the port when there are none yet.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.load()
	return m.st
}

func (m *Manager) load() {
	if m.loaded {
		return
	}
	m.loaded = true
	_ = store.LoadJSON(m.statePath(), &m.st)
	changed := false
	if m.st.ClientKey == "" {
		m.st.ClientKey, changed = randomKey("mc-"), true
	}
	if m.st.ManagementKey == "" {
		m.st.ManagementKey, changed = randomKey("mm-"), true
	}
	if m.st.Port == 0 {
		m.st.Port, changed = freePort(), true
	}
	if changed {
		_ = store.SaveJSON(m.statePath(), m.st)
	}
}

// Update changes the state under the lock and saves it.
func (m *Manager) Update(fn func(*State)) State {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.load()
	fn(&m.st)
	_ = store.SaveJSON(m.statePath(), m.st)
	return m.st
}

// Installed reports whether the executable of the version in use is there.
func (m *Manager) Installed() bool {
	st := m.State()
	return st.Version != "" && fileExists(m.BinaryPath(st.Version))
}

// BaseURL is the root of the local server.
func (m *Manager) BaseURL() string { return "http://127.0.0.1:" + itoa(m.State().Port) }

// APIURL and APIKey are what a provider needs to use the server.
func (m *Manager) APIURL() string { return m.BaseURL() + "/v1" }
func (m *Manager) APIKey() string { return m.State().ClientKey }

func randomKey(prefix string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// freePort asks the system for a port nobody listens on.
func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 18317
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// PortFree reports whether nothing listens on the port of the state.
func (m *Manager) PortFree() bool {
	l, err := net.Listen("tcp", "127.0.0.1:"+itoa(m.State().Port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// ChoosePort moves the server to another free port.
func (m *Manager) ChoosePort() {
	m.Update(func(s *State) { s.Port = freePort() })
}
