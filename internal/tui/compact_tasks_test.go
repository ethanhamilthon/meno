package tui

import (
	"reflect"
	"strings"
	"testing"

	"meno/internal/api"
	"meno/internal/render"
)

func TestCompactHidesTaskEventsButKeepsCounterAndDialog(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.globalTasks = 2
	turns := []api.Turn{
		{ID: "user", Role: render.RoleUser, Content: "please check this task"},
		{ID: "task", Role: render.RoleUser, Kind: render.KindEvent, Content: "<event source=\"task\" from=\"t1\">\nTASK-EVENT-MARKER\n</event>"},
		{ID: "answer", Role: render.RoleAssistant, Content: "answer"},
	}
	for _, turn := range turns {
		a.st.Session.add(turn)
	}
	a.st.Settings.View = "full"
	if !strings.Contains(timelineText(a), "TASK-EVENT-MARKER") {
		t.Fatal("full view must show the task event")
	}
	a.st.Settings.View = "compact"
	got := a.logText()
	if strings.Contains(string(got.flat()), "TASK-EVENT-MARKER") || !strings.Contains(string(got.flat()), turns[0].Content) {
		t.Fatal("compact selection document must contain user text but no task event")
	}
	// Removing a hidden event must not change cached layout, selection rows or spacing.
	a.st.Session.turns = []render.Turn{turnFromWire(turns[0]), turnFromWire(turns[2])}
	if !reflect.DeepEqual(got, a.logText()) {
		t.Fatal("hidden task event changed the document layout")
	}
	row, _ := statusRow(t, a, scr, 120)
	if !strings.Contains(row, "2 tasks") {
		t.Fatalf("compact must keep task counter: %q", row)
	}
	a.runCommand("tasks")
	if a.st.Over != OverTasks {
		t.Fatal("compact must still allow the tasks dialog")
	}
}
