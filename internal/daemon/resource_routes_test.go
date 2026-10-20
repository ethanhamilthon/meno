package daemon

import (
	"net/http/httptest"
	"testing"
)

func TestAgentResourceRoutesReachBackend(t *testing.T) {
	s, e := New(Options{Home: t.TempDir(), Backend: NewFakeBackend(NewHub("b", 10))})
	if e != nil {
		t.Fatal(e)
	}
	paths := []string{"/v1/sessions/s/tasks", "/v1/settings", "/v1/providers", "/v1/mcp/servers/a", "/v1/fs/search?root=/tmp&q=foo", "/v1/debug/requests?session=s"}
	for _, path := range paths {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
}
