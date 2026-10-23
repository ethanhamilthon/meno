package agent

import (
	"strings"
	"testing"
)

func TestActivatedSkillsAreListedInSummary(t *testing.T) {
	head := []Turn{{Role: "assistant", Code: "cat /h/skills/a/SKILL.md"}, {Role: "assistant", Code: "ls"}}
	got := activatedSkills([]string{"/h/skills/a/SKILL.md", "/h/skills/b/SKILL.md"}, "", head)
	if len(got) != 1 || got[0] != "/h/skills/a/SKILL.md" {
		t.Fatalf("%v", got)
	}
	// A skill named only in the previous summary stays pinned.
	if got := activatedSkills([]string{"/x/SKILL.md"}, "see /x/SKILL.md", nil); len(got) != 1 {
		t.Fatalf("%v", got)
	}
	s := withActivatedSkills("sum", got)
	if !strings.Contains(s, "- /h/skills/a/SKILL.md") || !strings.HasPrefix(s, "sum") {
		t.Fatal(s)
	}
	// A reminder the model copied is not duplicated.
	if twice := withActivatedSkills(s, got); strings.Count(twice, activatedHeading) != 1 {
		t.Fatal(twice)
	}
	if withActivatedSkills("sum", nil) != "sum" {
		t.Fatal("no skills must not change the summary")
	}
}
