package modes

import (
	"reflect"
	"strings"
	"testing"
)

func TestCatalogHasTheThreeModes(t *testing.T) {
	list := Catalog()
	var names []string
	for _, m := range list {
		names = append(names, m.Name)
		if !strings.HasPrefix(Block(m), "Mode: "+m.Name+".") {
			t.Errorf("%s: the text must start with its name: %q", m.Name, Block(m))
		}
	}
	if !reflect.DeepEqual(names, []string{"brainstorm", "goal", "subagents"}) {
		t.Fatalf("%v", names)
	}
	g, _ := Find(list, "goal")
	if !g.Goal || g.Rounds != DefaultRounds || g.Only != nil || g.Allow != nil {
		t.Fatalf("goal: %+v", g)
	}
	s, _ := Find(list, "subagents")
	if s.Goal || !reflect.DeepEqual(s.Only, subagentGlobals) || !reflect.DeepEqual(s.Allow, []string{GroupAgents, GroupAsk, GroupGoal, GroupHelp}) {
		t.Fatalf("subagents: %+v", s)
	}
	if b, _ := Find(list, "brainstorm"); b.Goal || b.Only != nil || b.Allow != nil {
		t.Fatalf("brainstorm must not limit the tools: %+v", b)
	}
}

func TestComposeIntersectsTheToolsAndKeepsGoal(t *testing.T) {
	list := Catalog()
	sub, _ := Find(list, "subagents")
	brain, _ := Find(list, "brainstorm")
	goal, _ := Find(list, "goal")

	c := Compose([]Mode{sub, brain})
	if !reflect.DeepEqual(c.Only, subagentGlobals) || !reflect.DeepEqual(c.Allow, sub.Allow) || c.Goal {
		t.Fatalf("subagents limits, brainstorm does not: %+v", c)
	}
	if !strings.Contains(c.Text, "Mode: subagents.") || !strings.Contains(c.Text, "Mode: brainstorm.") {
		t.Fatalf("both texts: %q", c.Text)
	}
	c = Compose([]Mode{sub, goal})
	if !c.Goal || c.Rounds != DefaultRounds || !reflect.DeepEqual(c.Only, subagentGlobals) || !reflect.DeepEqual(c.Allow, sub.Allow) {
		t.Fatalf("%+v", c)
	}
	if c := Compose([]Mode{brain}); c.Only != nil || c.Allow != nil || c.Goal {
		t.Fatalf("%+v", c)
	}
	if c := Compose(nil); c.Only != nil || c.Allow != nil || c.Goal || c.Text != "" {
		t.Fatalf("empty set: %+v", c)
	}
}

func TestAllowedGroupHelpersAndChildPolicy(t *testing.T) {
	all := Compose(nil)
	if !all.Allows(GroupFiles) || all.AllowedGroups() != nil {
		t.Fatal("nil allow must mean all groups")
	}
	c := Compose(Catalog())
	if c.Allows(GroupFiles) || !c.Allows(GroupHelp) {
		t.Fatalf("intersection should keep only subagent groups: %v", c.AllowedGroups())
	}
	got := c.AllowedGroups()
	got[0] = "changed"
	if c.AllowedGroups()[0] == "changed" {
		t.Fatal("allowed groups must be copied")
	}
	child := Compose([]Mode{{Allow: ChildAllow()}})
	for _, group := range []string{GroupFiles, GroupMCP, GroupWeb, GroupView, GroupModels, GroupHelp} {
		if !child.Allows(group) {
			t.Errorf("child should allow %q", group)
		}
	}
	for _, group := range []string{GroupAsk, GroupTasks} {
		if child.Allows(group) {
			t.Errorf("child should deny %q", group)
		}
	}
}
