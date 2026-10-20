package daemon

import (
	"encoding/json"
	"meno/internal/proto"
	"net/http/httptest"
	"testing"
)

func TestHelloProtocolMismatchRejectedByClient(t *testing.T) {
	_, e := New(Options{Home: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	h := proto.Hello{Protocol: proto.Protocol + 1}
	w := httptest.NewRecorder()
	writeJSON(w, 200, h)
	var got proto.Hello
	if e = json.Unmarshal(w.Body.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if got.Protocol == proto.Protocol {
		t.Fatal("test response does not mismatch")
	}
}
