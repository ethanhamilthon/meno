package llm

import "testing"

func TestPartialCode(t *testing.T) {
	full := `{"code":"const a = \"x\";\nprint(a) // é \u00e9 \ud83d\ude00"}`
	want := "const a = \"x\";\nprint(a) // é é 😀"
	if got := PartialCode(full); got != want {
		t.Fatalf("full: %q", got)
	}
	// Every prefix must yield a prefix of the final code and never panic.
	for i := 0; i <= len(full); i++ {
		got := PartialCode(full[:i])
		if len(got) > len(want) || want[:len(got)] != got {
			t.Fatalf("prefix %d: %q", i, got)
		}
	}
	if PartialCode(`{"co`) != "" || PartialCode(`{"code": `) != "" {
		t.Fatal("expected empty")
	}
}

func TestResponsesToolDeltas(t *testing.T) {
	s := responsesStream{items: map[string]any{}}
	var got []Event
	emit := func(e Event) { got = append(got, e) }
	for _, d := range []string{
		`{"type":"response.output_item.added","item":{"id":"fc1","type":"function_call","call_id":"c1","name":"js"}}`,
		`{"type":"response.function_call_arguments.delta","item_id":"fc1","delta":"{\"code\":\"ab"}`,
		`{"type":"response.function_call_arguments.delta","item_id":"fc1","delta":"c\"}"}`,
	} {
		if err := s.handle(SSEEvent{Data: d}, emit); err != nil {
			t.Fatal(err)
		}
	}
	if len(got) != 3 || got[2].Kind != EvToolDelta || got[2].CallID != "c1" || got[2].Code != "abc" {
		t.Fatalf("%+v", got)
	}
}
