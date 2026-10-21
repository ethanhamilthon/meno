package mcp

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseCommand(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{`plain --flag value`, []string{"plain", "--flag", "value"}},
		{`"/path with spaces/tool" 'hello world' "" ''`, []string{"/path with spaces/tool", "hello world", "", ""}},
		{`tool pre"mid dle"'tail' escaped\ space`, []string{"tool", "premid dletail", "escaped space"}},
		{`tool "a\"b" 'a\b' "a\qb"`, []string{"tool", `a"b`, `a\b`, `a\qb`}},
		{`tool $HOME $(echo nope) ; *.go`, []string{"tool", "$HOME", "$(echo", "nope)", ";", "*.go"}},
		{"tool\targ\nnext", []string{"tool", "arg", "next"}},
		{"tool ab\\\ncd", []string{"tool", "abcd"}},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseCommand(tc.in)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v %v; want %#v", got, err, tc.want)
			}
		})
	}
	for _, in := range []string{"", " \t\n", `"" arg`, `tool 'open`, `tool "open`, "tool trailing\\"} {
		if _, err := parseCommand(in); err == nil {
			t.Errorf("accepted malformed command %q", in)
		}
		if err := ValidateServer(Server{Name: "test", Transport: TransportStdio, Command: in}); err == nil {
			t.Errorf("validation accepted malformed command %q", in)
		}
	}
}

func assertNoPending(t *testing.T, c *conn) {
	t.Helper()
	c.pending.Range(func(key, value any) bool {
		t.Errorf("pending request leaked: %v", key)
		return true
	})
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }

func TestPendingSendFailure(t *testing.T) {
	for _, writer := range []io.Writer{failingWriter{}, shortWriter{}, io.Discard} {
		c := newConn(writer)
		for i := 0; i < 50; i++ {
			params := any(map[string]string{"ok": "value"})
			if writer == io.Discard {
				params = make(chan int) // Marshal failure must also release the ID.
			}
			if _, _, _, err := c.call("test", params); err == nil {
				t.Fatal("send unexpectedly succeeded")
			}
			assertNoPending(t, c)
		}
	}
}

func TestPendingWaitCleanup(t *testing.T) {
	for _, mode := range []string{"timeout", "cancel", "exit", "reply", "rpc-error"} {
		t.Run(mode, func(t *testing.T) {
			c := &Client{conn: newConn(io.Discard), done: make(chan struct{})}
			for i := 0; i < 50; i++ {
				_, ch, id, err := c.conn.call("test", nil)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				timeout := time.Second
				switch mode {
				case "timeout":
					timeout = time.Nanosecond
				case "cancel":
					cancel()
				case "exit":
					if i == 0 {
						close(c.done)
					}
				case "reply":
					ch <- rpcMsg{Result: []byte(`{}`)}
				case "rpc-error":
					ch <- rpcMsg{Error: &rpcError{Code: -1, Message: "failed"}}
				}
				_, err = c.wait(ctx, ch, id, timeout, "test")
				cancel()
				if (err == nil) != (mode == "reply") {
					t.Fatalf("wait: %v", err)
				}
				if mode == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				assertNoPending(t, c.conn)
			}
		})
	}
}

func TestConcurrentCancellationAndLateResponses(t *testing.T) {
	c := &Client{conn: newConn(io.Discard), done: make(chan struct{})}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ch, id, err := c.conn.call("test", nil)
			if err != nil {
				t.Error(err)
				return
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, _ = c.wait(ctx, ch, id, time.Second, "test")
		}()
	}
	wg.Wait()
	if err := c.conn.dispatch(strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":{}}` + "\n")); err != nil {
		t.Fatal(err)
	}
	assertNoPending(t, c.conn)
}
