package client

import (
	"context"
	"errors"
	"fmt"
	"meno/internal/api"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGlobalWatchRecoversOverflowAndStopsOnBye(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		if n == 1 {
			fmt.Fprint(w, "event: hello\ndata: {\"boot_id\":\"b\",\"seq\":8}\n\nid: 8\nevent: resync\ndata: {}\n\n")
			return
		}
		if r.URL.Query().Get("after") != "8" {
			t.Errorf("recovery cursor %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, "event: hello\ndata: {\"boot_id\":\"b\",\"seq\":8}\n\nid: 9\nevent: global_status\ndata: {}\n\nid: 10\nevent: bye\ndata: {}\n\n")
	}))
	defer server.Close()
	c := &Client{HTTP: &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}}}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	hello, status, bye := 0, 0, 0
	if err := c.WatchGlobal(ctx, func(e api.Event) error {
		switch e.Type {
		case api.EventHello:
			hello++
		case api.EventGlobalStatus:
			status++
		case api.EventBye:
			bye++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if hello != 2 || status != 1 || bye != 1 || hits.Load() != 2 {
		t.Fatalf("hello=%d status=%d bye=%d hits=%d", hello, status, bye, hits.Load())
	}
}

func TestGlobalWatchPreservesConsumerErrors(t *testing.T) {
	var hits int32
	c := sseServer(t, &hits, "event: hello\ndata: {\"boot_id\":\"b\",\"seq\":0}\n\n")
	want := errors.New("stop consumer")
	if err := c.WatchGlobal(context.Background(), func(api.Event) error { return want }); !errors.Is(err, want) {
		t.Fatalf("%v", err)
	}
	if hits != 1 {
		t.Fatal("consumer failure retried")
	}
}

func TestGlobalWatchRetriesHTTPFailure(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, "event: hello\ndata: {\"boot_id\":\"new\",\"seq\":0}\n\nid: 1\nevent: bye\ndata: {}\n\n")
	}))
	defer server.Close()
	c := &Client{HTTP: &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}}}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	unavailable, hellos := 0, 0
	err := c.WatchGlobal(ctx, func(e api.Event) error {
		if e.Type == GlobalUnavailable {
			unavailable++
		}
		if e.Type == api.EventHello {
			hellos++
		}
		return nil
	})
	if err != nil || unavailable != 1 || hellos != 1 || hits.Load() != 2 {
		t.Fatalf("err=%v unavailable=%d hello=%d hits=%d", err, unavailable, hellos, hits.Load())
	}
}
