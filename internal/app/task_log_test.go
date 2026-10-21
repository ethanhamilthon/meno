package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func logApp(t *testing.T, lines int) *App {
	t.Helper()
	a, _ := newTestApp(t)
	a.st.TasksUI.LogID = "t1"
	for i := 0; i < lines; i++ {
		a.st.TasksUI.Logs = append(a.st.TasksUI.Logs, taskLogLine{Text: fmt.Sprintf("line %03d", i)})
	}
	return a
}

func TestTaskLogScrollsAndFollows(t *testing.T) {
	a := logApp(t, 200)
	a.taskLogKey(tcell.NewEventKey(tcell.KeyPgUp, 0, 0))
	if a.st.TasksUI.Up == 0 {
		t.Fatal("PgUp must leave follow mode")
	}
	up := a.st.TasksUI.Up
	a.st.TasksUI.Logs = append(a.st.TasksUI.Logs, taskLogLine{Text: "new output"})
	if a.st.TasksUI.Up != up {
		t.Fatal("new output must not move a view scrolled away from the end")
	}
	a.taskLogKey(tcell.NewEventKey(tcell.KeyEnd, 0, 0))
	if a.st.TasksUI.Up != 0 {
		t.Fatal("End returns to follow mode")
	}
}

func TestTaskLogWrapsLongLinesInsteadOfCuttingThem(t *testing.T) {
	a := logApp(t, 0)
	a.st.TasksUI.Logs = []taskLogLine{{Text: strings.Repeat("abcdefghij ", 20)}}
	rows := a.taskLogRows(30)
	if len(rows) < 3 {
		t.Fatalf("a long line must wrap to several rows, got %d", len(rows))
	}
	var letters strings.Builder
	for _, r := range rows {
		letters.WriteString(strings.ReplaceAll(r.text, " ", ""))
	}
	want := strings.ReplaceAll(a.st.TasksUI.Logs[0].Text, " ", "")
	if letters.String() != want {
		t.Fatalf("text was lost while wrapping:\n got %q\nwant %q", letters.String(), want)
	}
}
