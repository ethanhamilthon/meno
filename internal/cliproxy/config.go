package cliproxy

import (
	"fmt"
	"os"
	"strings"
)

// WriteConfig writes config.yaml for the helper: it listens on the loopback
// only, answers to the client key of this meno, and keeps its tokens in the
// private auth directory. The helper hashes the management key in the file
// when it starts, so the file is written again before every start.
func (m *Manager) WriteConfig() error {
	st := m.State()
	if err := os.MkdirAll(m.AuthDir(), 0o700); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("config-version: 8\n")
	b.WriteString("server:\n  host: \"127.0.0.1\"\n")
	fmt.Fprintf(&b, "  port: %d\n", st.Port)
	b.WriteString("management:\n  allow-remote: false\n  disable-control-panel: true\n")
	fmt.Fprintf(&b, "  secret-key: %s\n", yamlQuote(st.ManagementKey))
	fmt.Fprintf(&b, "access:\n  api-keys:\n    - %s\n", yamlQuote(st.ClientKey))
	fmt.Fprintf(&b, "oauth:\n  auth-dir: %s\n", yamlQuote(m.AuthDir()))
	return os.WriteFile(m.ConfigPath(), []byte(b.String()), 0o600)
}

// Command is the shell command that runs the helper in the foreground, for
// proc.Daemons. Call WriteConfig before starting it.
func (m *Manager) Command() string {
	st := m.State()
	return shellQuote(m.BinaryPath(st.Version)) + " -config " + shellQuote(m.ConfigPath())
}
