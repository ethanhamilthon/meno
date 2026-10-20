package daemon

import (
	"context"
	"encoding/json"
	"meno/internal/proto"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func startTestServer(t *testing.T, home string, opts Options) (*Server, *http.Client) {
	t.Helper()
	opts.Home = home
	s, e := New(opts)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()
	t.Cleanup(func() { cancel(); s.Stop("test"); <-done })
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", s.Socket())
	}}
	c := &http.Client{Transport: tr}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, e := c.Get("http://unix/v1/hello")
		if e == nil {
			resp.Body.Close()
			return s, c
		}
		select {
		case err := <-done:
			done <- err
			t.Fatalf("server exited before readiness: %v", err)
		case <-time.After(5 * time.Millisecond):
		}
	}
	t.Fatal("server did not start")
	return nil, nil
}
func TestServerLifecycleAndModes(t *testing.T) {
	home := t.TempDir()
	hub := NewHub("", 20)
	backend := NewFakeBackend(hub)
	s, c := startTestServer(t, home, Options{Backend: backend, Hub: hub, Version: "test"})
	info, e := os.Stat(filepath.Join(home, "run"))
	if e != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("run dir mode: %v %v", info, e)
	}
	sock, e := os.Stat(s.Socket())
	if e != nil || sock.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode: %v %v", sock, e)
	}
	resp, e := c.Get("http://unix/v1/hello")
	if e != nil {
		t.Fatal(e)
	}
	var h proto.Hello
	_ = json.NewDecoder(resp.Body).Decode(&h)
	resp.Body.Close()
	if h.Protocol != proto.Protocol {
		t.Fatalf("protocol: %d", h.Protocol)
	}
}
func TestIdleExit(t *testing.T) {
	s, _ := startTestServer(t, t.TempDir(), Options{IdleExit: 1100 * time.Millisecond})
	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
		t.Fatal("idle daemon did not exit")
	}
}
func TestStaleFilesAndLongSocketFallback(t *testing.T) {
	home := t.TempDir()
	run := filepath.Join(home, "run")
	_ = os.MkdirAll(run, 0o700)
	_ = os.WriteFile(filepath.Join(run, "daemon.pid"), []byte("99999999"), 0o600)
	_ = os.WriteFile(filepath.Join(run, "meno.sock"), []byte("stale"), 0o600)
	if len(SocketPath(filepath.Join(home, string(make([]byte, 120))))) >= 100 {
		t.Fatal("long path fallback not short")
	}
	s, _ := startTestServer(t, home, Options{})
	s.Stop("done")
}
