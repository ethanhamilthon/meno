package client

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/db"
	"meno/internal/llm"
)

func TestIdleLifecycleRestoresPersistedSessions(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "meno")
	build := exec.Command("go", "build", "-o", bin, "./cmd/meno")
	build.Dir = "../.."
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, out)
	}
	saved := agent.NewSession(home, "fake")
	saved.Append(agent.Turn{Role: llm.RoleUser, Text: "survives automatic idle exit"})
	if e := agent.SaveSession(home, saved); e != nil {
		t.Fatal(e)
	}
	defer db.CloseAll()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 6*time.Second)
		defer stop()
		if _, e := New(home).Hello(cleanup); e == nil {
			lock, e := StopConfirmed(cleanup, home, false)
			if e != nil {
				t.Error(e)
			} else {
				lock.Close()
			}
		}
	})
	c, e := Ensure(ctx, Options{Home: home, Executable: bin})
	if e != nil {
		t.Fatal(e)
	}
	first, e := c.Hello(ctx)
	if e != nil {
		t.Fatal(e)
	}
	// Do not probe HTTP during the grace period: finite requests count as clients.
	deadline := time.Now().Add(14 * time.Second)
	for {
		_, e = os.Stat(c.Socket)
		if os.IsNotExist(e) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon did not exit with no attached clients")
		}
		time.Sleep(40 * time.Millisecond)
	}
	if _, e = os.Stat(filepath.Join(home, "run", "daemon.pid")); !os.IsNotExist(e) {
		t.Fatalf("PID file retained: %v", e)
	}
	c, e = Ensure(ctx, Options{Home: home, Executable: bin})
	if e != nil {
		t.Fatal(e)
	}
	next, e := c.Hello(ctx)
	if e != nil || next.BootID == first.BootID || next.PID == first.PID {
		t.Fatalf("restart: %+v %v", next, e)
	}
	rows, e := c.ListSessions(ctx, api.SessionsListRequest{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range rows.Sessions {
		if s.ID == saved.ID {
			testACPIdlePresence(t, home, bin, next.BootID)
			return
		}
	}
	t.Fatal("saved session missing")
}

// The ACP stdio connection must count as attached before session/new is called.
func testACPIdlePresence(t *testing.T, home, bin, boot string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 16*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "acp")
	cmd.Env = append(os.Environ(), "MENO_HOME="+home, "MENO_NO_METADATA=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		input.Close()
		if e := cmd.Wait(); e != nil {
			t.Errorf("ACP exit: %v", e)
		}
	}()
	if _, err = io.WriteString(input, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result json.RawMessage
		Error  json.RawMessage
	}
	if err = json.NewDecoder(output).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Result) == 0 || len(response.Error) > 0 {
		t.Fatalf("ACP initialize: %+v", response)
	}
	time.Sleep(11 * time.Second)
	hello, err := New(home).Hello(ctx)
	if err != nil || hello.BootID != boot {
		t.Fatalf("attached ACP lost daemon: %+v %v", hello, err)
	}
}
