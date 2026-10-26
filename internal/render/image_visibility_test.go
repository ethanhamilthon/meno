package render

import (
	"reflect"
	"strings"
	"testing"
)

func TestViewImageDeliveryAddsNoTranscriptRows(t *testing.T) {
	for _, view := range []ViewMode{ViewFull, ViewBashOnly, ViewCompact} {
		opts := JSOpts{View: view}
		user := Turn{Role: RoleUser, Text: "look [Image #1]"}
		image := Turn{Role: RoleUser, Kind: KindView, Text: "Image from view(\"x.png\"): [Image #1]"}
		answer := Turn{Role: RoleAssistant, Text: "answer"}
		got := BuildLines([]Turn{image, user, image, image, answer, image}, 100, opts)
		want := BuildLines([]Turn{user, answer}, 100, opts)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("view %d: synthetic image delivery must not change layout:\n%s", view, summaryText(got))
		}
		if lines := BuildLines([]Turn{image}, 100, opts); len(lines) != 0 {
			t.Fatalf("view %d: image-only delivery must have no rows", view)
		}
	}
}

func TestViewCommandUsesOrdinaryToolOutputRules(t *testing.T) {
	turn := Turn{Role: RoleAssistant, Code: "meno view x.png", Output: "image displayed"}
	for _, view := range []ViewMode{ViewFull, ViewBashOnly, ViewCompact} {
		text := summaryText(BuildLines([]Turn{turn}, 100, JSOpts{View: view}))
		if strings.Contains(text, "image displayed") != (view == ViewFull) || strings.Contains(text, "meno view") != (view != ViewCompact) {
			t.Fatalf("view %d: ordinary tool visibility changed: %q", view, text)
		}
	}
}
