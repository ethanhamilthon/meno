package proto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSessionVersionOrdering(t *testing.T) {
	a := SessionVersion{BootID: "b1", Revision: 3}
	if !(SessionVersion{BootID: "b1", Revision: 4}).Newer(a) || a.Newer(a) {
		t.Fatal("same boot must order by revision")
	}
	if (SessionVersion{BootID: "b2", Revision: 9}).Newer(a) {
		t.Fatal("different boots are not ordered")
	}
}

func TestSessionVersionWireFormat(t *testing.T) {
	b, _ := json.Marshal(SessionVersion{BootID: "x", Revision: 7})
	if string(b) != `{"boot_id":"x","revision":7}` {
		t.Fatalf("wire format changed: %s", b)
	}
}

// A fresh field is optional on the wire: a message from an older client has no
// origin and decodes as anonymous, and an anonymous one adds nothing to the JSON.
func TestOriginIsOptionalOnTheWire(t *testing.T) {
	var old MessageRequest
	if err := json.Unmarshal([]byte(`{"text":"hi","client_id":"c"}`), &old); err != nil || old.Origin != "" {
		t.Fatalf("old message: %+v %v", old, err)
	}
	b, _ := json.Marshal(MessageRequest{Text: "hi", ClientID: "c"})
	if strings.Contains(string(b), "origin") {
		t.Fatalf("anonymous message carries an origin: %s", b)
	}
	var st StatusData
	if err := json.Unmarshal([]byte(`{"running":true,"phase":"x"}`), &st); err != nil || st.Origin != "" {
		t.Fatalf("old status: %+v %v", st, err)
	}
}
