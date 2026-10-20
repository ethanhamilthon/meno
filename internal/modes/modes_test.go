package modes

import (
	"reflect"
	"strings"
	"testing"
)

func TestCatalogHasTheTwoModes(t *testing.T) {
	list := Catalog()
	var names []string
	for _, m := range list {
		names = append(names, m.Name)
		if !strings.HasPrefix(Block(m), "Mode: "+m.Name+".") {
			t.Errorf("%s: the text must start with its name: %q", m.Name, Block(m))
		}
	}
	if !reflect.DeepEqual(names, []string{"brainstorm", "goal"}) {
		t.Fatalf("%v", names)
	}
	g, _ := Find(list, "goal")
	if !g.Goal || g.Rounds != DefaultRounds || g.Only != nil || g.Allow != nil {
		t.Fatalf("goal: %+v", g)
	}
	if b, _ := Find(list, "brainstorm"); b.Goal || b.Only != nil || b.Allow != nil {
		t.Fatalf("brainstorm must not limit the tools: %+v", b)
	}
}

func TestComposeIntersectsTheToolsAndKeepsGoal(t *testing.T) {
	list := Catalog()
	sub := Mode{Name: "limited", Only: []string{"help"}, Allow: []string{GroupHelp}, Text: "Mode: limited."}
	brain, _ := Find(list, "brainstorm")
	goal, _ := Find(list, "goal")

	c := Compose([]Mode{sub, brain})
	if !reflect.DeepEqual(c.Only, sub.Only) || !reflect.DeepEqual(c.Allow, sub.Allow) || c.Goal {
		t.Fatalf("limited mode restricts tools: %+v", c)
	}
	if !strings.Contains(c.Text, "Mode: limited.") || !strings.Contains(c.Text, "Mode: brainstorm.") {
		t.Fatalf("both texts: %q", c.Text)
	}
	c = Compose([]Mode{sub, goal})
	if !c.Goal || c.Rounds != DefaultRounds || !reflect.DeepEqual(c.Only, sub.Only) || !reflect.DeepEqual(c.Allow, sub.Allow) {
		t.Fatalf("%+v", c)
	}
	if c := Compose([]Mode{brain}); c.Only != nil || c.Allow != nil || c.Goal {
		t.Fatalf("%+v", c)
	}
	if c := Compose(nil); c.Only != nil || c.Allow != nil || c.Goal || c.Text != "" {
		t.Fatalf("empty set: %+v", c)
	}
}

func TestAllowedGroupHelpers(t *testing.T) {
	all := Compose(nil)
	if !all.Allows(GroupFiles) || all.AllowedGroups() != nil {
		t.Fatal("nil allow must mean all groups")
	}
	c := Compose([]Mode{{Allow: []string{GroupHelp}}})
	if c.Allows(GroupFiles) || !c.Allows(GroupHelp) {
		t.Fatalf("restricted groups: %v", c.AllowedGroups())
	}
	got := c.AllowedGroups()
	got[0] = "changed"
	if c.AllowedGroups()[0] == "changed" {
		t.Fatal("allowed groups must be copied")
	}

}
