package jsvm

import (
	"strings"
	"testing"
)

func TestHelpListsTopicsAndReturnsText(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	r := ex(t, v, "help()")
	for _, name := range HelpTopics(false) {
		if !strings.Contains(r.Value, "- "+name+":") {
			t.Fatalf("topic %s is not listed:\n%s", name, r.Value)
		}
	}
	for _, name := range HelpTopics(false) {
		r := ex(t, v, "help('"+name+"')")
		if r.Err != "" || !strings.HasPrefix(r.Value, "# ") || len(r.Value) > 4500 {
			t.Fatalf("%s: err=%q len=%d", name, r.Err, len(r.Value))
		}
	}
}

func TestHelpUnknownTopicListsTheTopics(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	r := ex(t, v, "help('nope')")
	if !strings.Contains(r.Err, "unknown topic") || !strings.Contains(r.Err, "mcpServers") {
		t.Fatalf("%+v", r)
	}
	if r := ex(t, v, "help(5)"); !strings.Contains(r.Err, "must be a string") {
		t.Fatalf("%+v", r)
	}
}

func TestHelpTextsDoNotMentionRemovedThings(t *testing.T) {
	for _, child := range []bool{false, true} {
		for _, name := range HelpTopics(child) {
			text, err := helpText(name, child)
			if err != nil {
				t.Fatal(err)
			}
			for _, bad := range []string{"state.", "sessions.", "session.compact", "saveLastNMessages"} {
				if strings.Contains(text, bad) {
					t.Errorf("topic %s (child=%v) mentions %q", name, child, bad)
				}
			}
		}
	}
}

func TestSubagentHelpHidesMainOnlyTopics(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	v.SetAgents(nil, true)
	for _, name := range []string{"askUser", "tasks"} {
		if r := ex(t, v, "help('"+name+"')"); !strings.Contains(r.Err, "unknown topic") {
			t.Fatalf("%s must be hidden: %+v", name, r)
		}
	}
	if r := ex(t, v, "help('agents')"); !strings.Contains(r.Value, "You are a subagent") {
		t.Fatalf("%+v", r)
	}
}
