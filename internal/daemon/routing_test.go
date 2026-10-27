package daemon

import (
	"meno/internal/api"
	"net/http/httptest"
	"testing"
)

func TestRoutesReturnStubNotImplemented(t *testing.T) {
	s := New(Options{Backend: NotImplementedBackend{}})
	r := httptest.NewRequest("GET", "/v1/sessions", nil)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != 501 {
		t.Fatalf("status %d", w.Code)
	}
	if !contains(w.Body.String(), api.CodeNotImplemented) {
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
