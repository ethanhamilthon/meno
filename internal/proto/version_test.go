package proto

import (
	"encoding/json"
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
