package mcp

import (
	"context"
	"os"
	"os/exec"
	"sync"
)

// stderrBuffer keeps the last bytes a child process wrote to stderr. MCP
// servers report their startup problems there, and that text is the only
// useful clue when the handshake fails.
type stderrBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

// stderrLimit is how much of a child's stderr is kept.
const stderrLimit = 8 << 10

func newStderrBuffer() *stderrBuffer { return &stderrBuffer{max: stderrLimit} }

// Write keeps the tail of the stream.
func (b *stderrBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.max {
		b.buf = append([]byte(nil), b.buf[len(b.buf)-b.max:]...)
	}
	return len(p), nil
}

// String returns the kept output.
func (b *stderrBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// stdioCommand builds the command line of a stdio server: the words of Command
// followed by Args. The usual MCP shape ("command": "python", "args": ["s.py"])
// gives "python s.py".
func stdioCommand(s Server) ([]string, error) {
	parts, err := parseCommand(s.Command)
	if err != nil {
		return nil, err
	}
	return append(parts, s.Args...), nil
}

// stdioEnv returns the environment of a stdio server. It sets
// PYTHONUNBUFFERED=1 unless the server sets it itself: a Python server that
// writes to a pipe block-buffers its output and never answers the handshake.
func stdioEnv(s Server) []string {
	env := os.Environ()
	for k, v := range s.Env {
		env = append(env, k+"="+v)
	}
	if _, ok := s.Env["PYTHONUNBUFFERED"]; !ok {
		env = append(env, "PYTHONUNBUFFERED=1")
	}
	return env
}

// stdioProcess prepares the child process of a stdio server.
func stdioProcess(ctx context.Context, s Server) (*exec.Cmd, *stderrBuffer, error) {
	parts, err := stdioCommand(s)
	if err != nil {
		return nil, nil, err
	}
	errs := newStderrBuffer()
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Env = stdioEnv(s)
	cmd.Dir = s.Cwd
	cmd.Stderr = errs
	return cmd, errs, nil
}
