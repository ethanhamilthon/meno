package llm

import "testing"

// A request built from the same history twice is identical, and adding a
// message keeps every earlier byte: the prefix cache has something to hit.
func TestRequestPrefixIsCanonical(t *testing.T) {
	msgs := []Msg{
		{Role: RoleUser, Text: "hello"},
		{Role: RoleAssistant, Text: "hi", CallID: "c1", Code: "1+1"},
		{Role: RoleTool, CallID: "c1", Text: "2"},
	}
	a := FingerprintOf(Request{Model: "m", System: "sys", Msgs: msgs, Tools: true})
	b := FingerprintOf(Request{Model: "m", System: "sys", Msgs: append([]Msg(nil), msgs...), Tools: true})
	if a.SharedPrefix(b) != 3 {
		t.Fatalf("identical history must be identical: %+v %+v", a, b)
	}
	grown := FingerprintOf(Request{Model: "m", System: "sys", Msgs: append(append([]Msg(nil), msgs...), Msg{Role: RoleUser, Text: "next"}), Tools: true})
	if grown.SharedPrefix(a) != 3 || grown.Messages != 4 {
		t.Fatalf("appending must keep the prefix: %d", grown.SharedPrefix(a))
	}
	// Changing an early message, the system prompt or the model breaks sharing.
	edited := append([]Msg(nil), msgs...)
	edited[1].Text = "hi!"
	if got := FingerprintOf(Request{Model: "m", System: "sys", Msgs: edited, Tools: true}).SharedPrefix(a); got != 1 {
		t.Fatalf("edit at message 1 must share exactly 1 message, got %d", got)
	}
	if FingerprintOf(Request{Model: "m", System: "sys2", Msgs: msgs, Tools: true}).SharedPrefix(a) != -1 {
		t.Fatal("a different system prompt shares nothing")
	}
	if FingerprintOf(Request{Model: "m2", System: "sys", Msgs: msgs, Tools: true}).SharedPrefix(a) != -1 {
		t.Fatal("a different model shares nothing")
	}
	// A fingerprint carries no text.
	if a.SystemLen != 3 || len(a.SystemHash) != 8 {
		t.Fatalf("%+v", a)
	}
}

// The wire bodies of both protocols are deterministic: the same request
// serializes to the same bytes, and a longer history extends the earlier one.
func TestWireBodiesAreDeterministicAndPrefixStable(t *testing.T) {
	msgs := []Msg{{Role: RoleUser, Text: "a"}, {Role: RoleAssistant, Text: "b", CallID: "c", Code: "x"}, {Role: RoleTool, CallID: "c", Text: "r"}}
	for i := 0; i < 20; i++ {
		x, y := mustJSON(chatMessages("s", msgs)), mustJSON(chatMessages("s", msgs))
		if x != y {
			t.Fatal("chat body differs between builds")
		}
		rx, ry := mustJSON(responsesInput(msgs)), mustJSON(responsesInput(msgs))
		if rx != ry {
			t.Fatal("responses body differs between builds")
		}
	}
	short, long := mustJSON(chatMessages("s", msgs[:2])), mustJSON(chatMessages("s", msgs))
	if long[:len(short)-1] != short[:len(short)-1] {
		t.Fatalf("a longer history must extend the shorter one byte for byte")
	}
	rshort, rlong := mustJSON(responsesInput(msgs[:2])), mustJSON(responsesInput(msgs))
	if rlong[:len(rshort)-1] != rshort[:len(rshort)-1] {
		t.Fatal("responses: a longer history must extend the shorter one")
	}
}
