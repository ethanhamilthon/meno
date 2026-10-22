package tui

import (
	"strings"
	"testing"
)

func TestBackgroundCountIncludesOnlyActiveTasks(t *testing.T) {
	a, rt := queueTestApp()
	a.rtList = []*Runtime{rt}
	rt.Tasks = []taskRow{{Status: "starting"}, {Status: "running"}, {Status: "done"}, {Status: "failed"}, {Status: "blocked"}}
	if n := a.bgCount(rt.ID()); n != 2 {
		t.Fatalf("background count %d, want 2", n)
	}
	if !a.bgAny() || a.bgCount("missing") != 0 {
		t.Fatal("background state does not match the task list")
	}
	rt.Tasks = []taskRow{{Status: "done"}}
	if a.bgAny() {
		t.Fatal("completed tasks kept the background marker active")
	}
}

func TestRemovedAgentFeaturesAreNotExposed(t *testing.T) {
	for _, tab := range settingTabs {
		if tab.name == "Agents" {
			t.Fatal("the removed settings tab is still visible")
		}
		for _, row := range tab.rows {
			if strings.HasPrefix(row.key, "subagent_") {
				t.Fatalf("removed setting is still visible: %s", row.key)
			}
		}
	}
	for _, cmd := range commandList {
		if cmd.name == ":agents" {
			t.Fatal("the removed command is still suggested")
		}
	}
	for _, row := range helpRows {
		if row.key == ":agents" || strings.Contains(row.key, "#subagents") {
			t.Fatalf("removed feature is still in help: %s", row.key)
		}
	}
	a, _ := queueTestApp()
	a.runCommand("agents")
	if a.st.Over != OverNone || len(a.st.Notices) == 0 || !a.st.Notices[len(a.st.Notices)-1].Err {
		t.Fatal("the removed command is still accepted")
	}
}
