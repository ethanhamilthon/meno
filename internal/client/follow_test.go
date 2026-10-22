package client

import (
	"context"
	"fmt"
	"meno/internal/api"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFollowMetadataTasksAndCompletionNeedOneSnapshot(t *testing.T) {
	var snapshots atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/hello":
			fmt.Fprintf(w, `{"protocol":%d,"boot_id":"b"}`, api.Protocol)
		case "/v1/sessions/s":
			snapshots.Add(1)
			fmt.Fprint(w, `{"session":{"id":"s","cwd":"/"},"seq":1,"version":{"boot_id":"b","revision":1}}`)
		case "/v1/sessions/s/events":
			fmt.Fprint(w, "event: hello\ndata: {\"boot_id\":\"b\",\"seq\":1}\n\n")
			for _, e := range []api.Event{
				ev(2, api.EventSessionChanged, api.ChangedData{Fields: []string{"name", "cwd", "provider", "effort", "unread"}, Metadata: &api.SessionMetadata{Session: api.SessionItem{ID: "s", Name: "named", Cwd: "/new", Completed: 1}, Provider: "p"}}),
				ev(3, api.EventTasksChanged, api.TasksChangedData{Tasks: []api.Task{{ID: "t", Status: "completed"}}}),
				ev(4, api.EventDone, api.DoneData{}),
			} {
				fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Seq, e.Type, e.Data)
			}
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := &Client{HTTP: &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}}}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resets, updates := 0, 0
	err := c.Follow(ctx, "s", func(u ProjectionUpdate) error {
		if u.Reset {
			resets++
		} else {
			updates++
		}
		if u.Effects.Finished {
			cancel()
		}
		return nil
	})
	if err != context.Canceled {
		t.Fatalf("%v", err)
	}
	if snapshots.Load() != 1 || resets != 1 || updates != 3 {
		t.Fatalf("snapshots=%d resets=%d updates=%d", snapshots.Load(), resets, updates)
	}
}
