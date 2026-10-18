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
		if !strings.Contains(out, "Do not use async, await, Promise, or then") {
			t.Errorf("subagent=%v: prompt must forbid async/await", sub)
		}
		if strings.Contains(out, "await works on") {
			t.Errorf("subagent=%v: prompt still allows await", sub)
		}
	}
}

func TestNoRenameRule(t *testing.T) {
	for _, sub := range []bool{false, true} {
		in := full()
		in.Subagent = sub
		out := Build(in)
		for _, bad := range []string{"session.rename", "todo", "- Plan:"} {
			if strings.Contains(out, bad) {
				t.Errorf("subagent=%v: prompt still mentions %q", sub, bad)
			}
		}
	}
}

func TestGuidelinesHoldAskRule(t *testing.T) {
	main := Build(full())
	for _, want := range []string{"- Ask: use askUser(", "help(\"askUser\")"} {
		if !strings.Contains(main, want) {
			t.Errorf("missing %q", want)
		}
	}
	in := full()
	in.Subagent = true
	child := Build(in)
	if strings.Contains(child, "- Ask:") || strings.Contains(child, "askUser") {
		t.Error("a subagent has no ask rule and no askUser")
	}
	if strings.Contains(child, "tasks.start") || strings.Contains(child, "agents.spawn") {
		t.Error("a subagent has no tasks or agents.spawn")
	}
}

func TestPromptDoesNotMentionRemovedThings(t *testing.T) {
	for _, sub := range []bool{false, true} {
		in := full()
		in.Subagent = sub
		out := Build(in)
		for _, bad := range []string{"state.", "sessions.", "session.compact", "retrievable", "## State", "## Session"} {
			if strings.Contains(out, bad) {
				t.Errorf("subagent=%v: prompt mentions %q", sub, bad)
			}
		}
		if !strings.Contains(out, "help() lists the topics") {
			t.Errorf("subagent=%v: prompt must explain help", sub)
		}
	}
}
