package render

import (
	"reflect"
	"strings"
	"testing"
)

func TestCompactHidesOnlyTaskEventsWithoutGaps(t *testing.T) {
	task := Turn{Role: RoleUser, Kind: KindEvent, Text: "<event id=\"t1\" source=\"task\" from=\"t1\" kind=\"task_done\">\nfinished-marker\n</event>"}
	system := Turn{Role: RoleUser, Kind: KindEvent, Text: "<event id=\"s1\" source=\"system\">\nkeep-marker\n</event>"}
	user := Turn{Role: RoleUser, Text: "please check this task"}
	answer := Turn{Role: RoleAssistant, Text: "answer"}
	for _, view := range []ViewMode{ViewFull, ViewBashOnly, ViewCompact} {
		opts := JSOpts{View: view}
		got := BuildLines([]Turn{user, task, task, system, answer, task}, 100, opts)
		if view == ViewCompact {
			want := BuildLines([]Turn{user, system, answer}, 100, opts)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("hidden tasks must not change layout:\n%s", summaryText(got))
			}
			if lines := BuildLines([]Turn{task, task}, 100, opts); len(lines) != 0 {
				t.Fatalf("task-only history must have no rows: %+v", lines)
			}
		} else if !strings.Contains(summaryText(got), "task t1: finished-marker") {
			t.Fatalf("view %d lost task event", view)
		}
		if !strings.Contains(summaryText(got), user.Text) || !strings.Contains(summaryText(got), "keep-marker") {
			t.Fatal("non-task content was hidden")
		}
	}
}

func TestCompactDoesNotReadEventSourceFromBody(t *testing.T) {
	turn := Turn{Role: RoleUser, Kind: KindEvent, Text: "<system-event><note>source=\"task\"</note></system-event>"}
	if len(BuildLines([]Turn{turn}, 80, JSOpts{View: ViewCompact})) == 0 {
		t.Fatal("a task reference inside another event is not a task event")
	}
}
