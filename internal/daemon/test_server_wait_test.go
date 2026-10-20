package daemon

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func waitForServer(t *testing.T, s *Server) {
	t.Helper()
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", s.Socket())
	}}
	c := &http.Client{Transport: tr}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, e := c.Get("http://unix/v1/hello"); e == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("server did not start")
}
