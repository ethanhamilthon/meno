package prompt

import (
	"strings"
	"testing"

	"meno/internal/jsvm"
)

func TestToolDescriptionOmitsRemovedEdit(t *testing.T) {
	d := ToolDescription(false)
	line := "fs: read, inspect, search, patch, write, glob, exists."
	if !strings.Contains(d, line) || strings.Contains(d, "fs: read, inspect, search, patch, edit") {
		t.Fatalf("filesystem tool list is wrong: %s", d)
	}
}

// Without a restriction the description is the one every request had before.
func TestToolDescriptionForNilIsUnchanged(t *testing.T) {
	if ToolDescriptionFor(false, nil) != ToolDescription(false) {
		t.Fatal("no restriction, no change: the prompt cache must not move")
	}
}

// A subagent-mode description names the globals it lost and the ones it keeps.
func TestToolDescriptionForNamesWhatIsGone(t *testing.T) {
	only := []string{"agents", "goal", "askUser", "help", "hash", "path", "base64", "btoa", "atob"}
	d := ToolDescriptionFor(false, only)
	if !strings.Contains(d, "ReferenceError") || !strings.Contains(d, "fs, sh") {
		t.Fatalf("the lost globals must be named: %s", d)
	}
	if !strings.Contains(d, "Only these remain: agents, goal") {
		t.Fatalf("the kept globals must be named: %s", d)
	}
	for _, n := range jsvm.ToolGlobals {
		if strings.Contains(d, "Only these remain: "+n+",") && n == "fs" {
			t.Fatal("fs is not kept")
		}
	}
}
