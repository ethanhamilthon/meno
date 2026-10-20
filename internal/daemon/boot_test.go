package daemon

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestServerBootIDsAreUniqueAndStreamResyncs(t *testing.T) {
	home := t.TempDir()
	s1, e := New(Options{Home: home})
	if e != nil {
		t.Fatal(e)
	}
	boot1 := s1.BootID()
	if _, e = New(Options{Home: filepath.Join(home, "two")}); e != nil {
		t.Fatal(e)
	}
	s2, e := New(Options{Home: filepath.Join(home, "three")})
	if e != nil {
		t.Fatal(e)
	}
	if boot1 == s2.BootID() {
		t.Fatal("boot IDs are not unique")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s1.Serve(ctx) }()
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", s1.Socket())
	}}
	c := &http.Client{Transport: tr}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, e = c.Get("http://unix/v1/hello"); e == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	hub := s1.hub
	hub.Publish("s", "text_delta", map[string]string{"text": "x"})
	ch, done, resync := hub.Subscribe("s", 0, "old-boot")
	defer done()
	if !resync {
		t.Fatal("boot change did not resync")
	}
	if e := <-ch; e.Type != "resync" {
		t.Fatalf("event %s", e.Type)
	}
	s1.Stop("done")

}
