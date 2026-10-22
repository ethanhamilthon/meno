package daemon

import (
	"bufio"
	"meno/internal/api"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSSELastEventIDAndQueryResume(t *testing.T) {
	for _, header := range []bool{true, false} {
		home := t.TempDir()
		hub := NewHub("b", 20)
		s, c := startTestServer(t, home, Options{Hub: hub})
		hub.Publish("session", api.EventTextDelta, api.TextDeltaData{Text: "one"})
		hub.Publish("session", api.EventTextDelta, api.TextDeltaData{Text: "two"})
		req, _ := http.NewRequest("GET", "http://unix/v1/sessions/session/events?after=0&boot_id="+s.BootID(), nil)
		if header {
			req.Header.Set("Last-Event-ID", "1")
		}
		resp, e := c.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		reader := bufio.NewReader(resp.Body)
		deadline := time.Now().Add(time.Second)
		seen := false
		for !seen && time.Now().Before(deadline) {
			line, e := reader.ReadString(10)
			if e != nil {
				t.Fatal(e)
			}
			seen = strings.Contains(line, "text_delta")
		}
		resp.Body.Close()
		if !seen {
			t.Fatal("resume event not received")
		}
		s.Stop("test")
	}
}
func TestSSEBootMismatchSendsResync(t *testing.T) {
	home := t.TempDir()
	hub := NewHub("boot", 10)
	_, c := startTestServer(t, home, Options{Hub: hub})
	hub.Publish("s", api.EventDone, map[string]bool{})
	resp, e := c.Get("http://unix/v1/sessions/s/events?after=0&boot_id=old")
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	b := make([]byte, 4096)
	n, _ := resp.Body.Read(b)
	text := string(b[:n])
	if !strings.Contains(text, "resync") {
		more, _ := resp.Body.Read(b[n:])
		text += string(b[n : n+more])
	}
	if !strings.Contains(text, "resync") {
		t.Fatalf("body %s", text)
	}
}
