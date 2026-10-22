package daemon

import (
	"encoding/json"
	"meno/internal/api"
	"net/http/httptest"
	"testing"
)

func TestHelloProtocolMismatchRejectedByClient(t *testing.T) {
	_, e := New(Options{Home: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	h := api.Hello{Protocol: api.Protocol + 1}
	w := httptest.NewRecorder()
	writeJSON(w, 200, h)
	var got api.Hello
	if e = json.Unmarshal(w.Body.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if got.Protocol == api.Protocol {
		t.Fatal("test response does not mismatch")
	}
}
