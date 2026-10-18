package prompt

import (
	"strings"
	"testing"
)

func TestPromptForbidsAsyncJS(t *testing.T) {
	for _, sub := range []bool{false, true} {
		in := full()
		in.Subagent = sub
		out := Build(in)
		if !strings.Contains(out, "Never use async, await, Promise or then") {
			t.Errorf("subagent=%v: prompt must forbid async/await", sub)
		}
		if strings.Contains(out, "await works on") {
			t.Errorf("subagent=%v: prompt still allows await", sub)
		}
	}
}

func TestMainAgentMustRename(t *testing.T) {
	out := Build(full())
	if !strings.Contains(out, "You must always call it") {
		t.Error("main agent prompt must require session.rename")
	}
	in := full()
	in.Subagent = true
	if strings.Contains(Build(in), "session.rename") {
		t.Error("subagents have no session.rename")
	}
}
