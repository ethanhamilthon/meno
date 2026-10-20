package daemon

import (
	"meno/internal/proto"
	"net/http/httptest"
	"testing"
)

func TestRoutesReturnStubNotImplemented(t *testing.T) {
	home := t.TempDir()
	s, e := New(Options{Home: home, Backend: NotImplementedBackend{}})
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "/v1/sessions", nil)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != 501 {
		t.Fatalf("status %d", w.Code)
	}
	if !contains(w.Body.String(), proto.CodeNotImplemented) {
		t.Fatalf("body %s", w.Body.String())
	}
}
func contains(s, part string) bool {
	return len(part) == 0 || len(s) >= len(part) && index(s, part) >= 0
}
func index(s, p string) int {
	for i := 0; i+len(p) <= len(s); i++ {
		if s[i:i+len(p)] == p {
			return i
		}
	}
	return -1
}
