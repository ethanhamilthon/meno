package llm

import "testing"

func TestBashTitleSchemaAndSafeFallback(t *testing.T) {
	p := BashSchema("")["parameters"].(map[string]any)
	required := p["required"].([]string)
	if len(required) != 2 || required[1] != "title" {
		t.Fatal(required)
	}
	for raw, want := range map[string]string{`{"command":"go test ./..."}`: "Run go", `{"command":"TOKEN=secret curl private"}`: "Bash command", `{"command":"echo secret","title":"  Check\n output "}`: "Check output"} {
		if got := BashTitle(raw); got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}
