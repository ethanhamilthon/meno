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
	if !g.Goal || g.Rounds != DefaultRounds || g.Only != nil {
		t.Fatalf("goal: %+v", g)
	}
	s, _ := Find(list, "subagents")
	if s.Goal || !reflect.DeepEqual(s.Only, subagentGlobals) {
		t.Fatalf("subagents: %+v", s)
	}
	if b, _ := Find(list, "brainstorm"); b.Goal || b.Only != nil {
		t.Fatalf("brainstorm must not limit the tools: %+v", b)
	}
}

func TestComposeIntersectsTheToolsAndKeepsGoal(t *testing.T) {
	list := Catalog()
	sub, _ := Find(list, "subagents")
	brain, _ := Find(list, "brainstorm")
	goal, _ := Find(list, "goal")

	c := Compose([]Mode{sub, brain})
	if !reflect.DeepEqual(c.Only, subagentGlobals) || c.Goal {
		t.Fatalf("subagents limits, brainstorm does not: %+v", c)
	}
	if !strings.Contains(c.Text, "Mode: subagents.") || !strings.Contains(c.Text, "Mode: brainstorm.") {
		t.Fatalf("both texts: %q", c.Text)
	}
	c = Compose([]Mode{sub, goal})
	if !c.Goal || c.Rounds != DefaultRounds || !reflect.DeepEqual(c.Only, subagentGlobals) {
		t.Fatalf("%+v", c)
	}
	if c := Compose([]Mode{brain}); c.Only != nil || c.Goal {
		t.Fatalf("%+v", c)
	}
	if c := Compose(nil); c.Only != nil || c.Goal || c.Text != "" {
		t.Fatalf("empty set: %+v", c)
	}
}
