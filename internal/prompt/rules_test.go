package prompt

import (
	"strings"
	"testing"
)

// both is the text the model sees: the system prompt and the tool description.
func both(in Input) string { return Build(in) + "\n" + ToolDescription(in.Subagent) }

func TestToolDescriptionForbidsAsyncJS(t *testing.T) {
	for _, sub := range []bool{false, true} {
		out := ToolDescription(sub)
		if !strings.Contains(out, "Do not use async, await, Promise, or then") {
			t.Errorf("subagent=%v: description must forbid async/await", sub)
		}
		if strings.Contains(out, "await works on") {
			t.Errorf("subagent=%v: description still allows await", sub)
		}
	}
}

func TestToolDescriptionExplainsTheRuntimeState(t *testing.T) {
	for _, sub := range []bool{false, true} {
		out := ToolDescription(sub)
		for _, want := range []string{"lives for your current answer", "A new message from the user starts a clean runtime"} {
			if !strings.Contains(out, want) {
				t.Errorf("subagent=%v: missing %q", sub, want)
			}
		}
		if strings.Contains(out, "Nothing survives") || strings.Contains(out, "clean runtime. ") {
			t.Errorf("subagent=%v: the old one-call-runtime wording is still there", sub)
		}
	}
}

func TestSystemPromptHoldsNoJSReference(t *testing.T) {
	for _, sub := range []bool{false, true} {
		in := full()
		in.Subagent = sub
		if sub {
			in.Only = []string{"agents", "goal", "askUser", "help", "hash", "path", "base64", "btoa", "atob"}
		}
		out := Build(in)
		for _, moved := range []string{"## The js tool", "fs: read, inspect", "sh(cmd) returns", "websearch(query", "mcp.<server>.<tool>", "tasks.start(", "agents.spawn(", "help() lists"} {
			if strings.Contains(out, moved) {
				t.Errorf("subagent=%v: %q belongs to the tool description", sub, moved)
			}
		}
		if !strings.Contains(out, "Your only tool is js") {
			t.Errorf("subagent=%v: the prompt must say that js is the only tool", sub)
		}
		if !strings.Contains(out, "<voice>") || !strings.Contains(out, "<method>") {
			t.Errorf("subagent=%v: missing voice or method section", sub)
		}
		if sub {
			if strings.Contains(out, "<editing>") || strings.Contains(out, "<verification>") || strings.Contains(out, "<output>") {
				t.Error("restricted prompt must omit tool-dependent sections")
			}
		} else if !strings.Contains(out, "<editing>") || !strings.Contains(out, "<verification>") || !strings.Contains(out, "<output>") {
			t.Error("unrestricted prompt must keep tool-dependent sections")
		}
	}
}

func TestNoRenameRule(t *testing.T) {
	for _, sub := range []bool{false, true} {
		in := full()
		in.Subagent = sub
		out := both(in)
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
	child := both(in)
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
		out := both(in)
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

func TestToolDescriptionHoldsTheJSReference(t *testing.T) {
	main := ToolDescription(false)
	for _, want := range []string{"fs: read, inspect, search, patch, write, glob, exists", "sh(cmd) returns", "websearch(query", "mcp.<server>.<tool>", "mcpTools(\"server\")", "tasks.start(", "agents.spawn(", "askUser", "help() lists the topics"} {
		if !strings.Contains(main, want) {
			t.Errorf("main: missing %q", want)
		}
	}
	child := ToolDescription(true)
	for _, bad := range []string{"tasks.start", "agents.spawn", "askUser", "Background tasks"} {
		if strings.Contains(child, bad) {
			t.Errorf("child: must not mention %q", bad)
		}
	}
	if !strings.Contains(child, "mcp.<server>.<tool>") || !strings.Contains(child, "agents") {
		t.Error("child: lost MCP or agents.report access")
	}
}

func TestToolDescriptionIsStable(t *testing.T) {
	for _, sub := range []bool{false, true} {
		first := ToolDescription(sub)
		for i := 0; i < 3; i++ {
			if ToolDescription(sub) != first {
				t.Fatal("the description feeds the prompt cache and must not vary")
			}
		}
	}
}
