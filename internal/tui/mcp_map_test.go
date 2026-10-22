package tui

import (
	"reflect"
	"testing"
)

func TestMCPMapRoundTrip(t *testing.T) {
	want := map[string]string{"Accept": "application/json, text/event-stream", "Authorization": "Bearer a=b", "SPACES": "  literal  ", "EMPTY": ""}
	got, err := parseMapJSON(mapJSON(want))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip %+v %v", got, err)
	}
	if _, err = parseMapJSON(`{"wrong":1}`); err == nil {
		t.Fatal("accepted non-string value")
	}
}
