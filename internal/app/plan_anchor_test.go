package app

import (
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/llm"
)

func rowOf(out, needle string) int {
	for i, l := range strings.Split(out, "\n") {
		if strings.Contains(l, needle) {
			return i
		}
	}
	return -1
}

func TestPlanDoesNotMoveWhileTheRunStreams(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Todo = &agent.Plan{Items: []agent.Item{{ID: "i1", Title: "step one", Status: agent.ItemInProgress}}}
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "old answer"})
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "next task"})
	a.st.PlanAt = a.st.Session.Len()
	a.st.LiveBase = a.st.Session.Len()
	a.st.LiveText = "streaming reply"
	before, _ := transcriptText(a)
	row := rowOf(before, "step one")
	if row < 0 || rowOf(before, "streaming reply") < row {
		t.Fatalf("plan must sit above the live reply:\n%s", before)
	}
	// The reply is saved and the live copy disappears: the plan stays.
	a.st.LiveText = ""
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "streaming reply"})
	after, _ := transcriptText(a)
	if got := rowOf(after, "step one"); got != row {
		t.Fatalf("plan moved from row %d to %d:\n%s", row, got, after)
	}
}

func TestPlanAnchorFallsBackToLastUserMessage(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	turns := []agent.Turn{{Role: llm.RoleUser, Text: "a"}, {Role: llm.RoleAssistant, Text: "b"}, {Role: llm.RoleUser, Text: "c"}, {Role: llm.RoleAssistant, Text: "d"}}
	if got := a.planAnchor(turns); got != 3 {
		t.Fatalf("anchor %d", got)
	}
	a.st.PlanAt = 99
	if got := a.planAnchor(turns); got != 3 {
		t.Fatalf("stale anchor not ignored: %d", got)
	}
}
