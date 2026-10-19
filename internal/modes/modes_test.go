package modes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltins(t *testing.T) {
	list := Load()
	if len(list) != 3 || list[0].Name != "build" || list[1].Name != "goal" || list[2].Name != "plan" {
		t.Fatalf("%+v", list)
	}
	plan, _ := Find(list, "plan")
	goal, _ := Find(list, "goal")
	build, _ := Find(list, "build")
	if !plan.ReadOnly || plan.Next != "build" || plan.Goal {
		t.Fatalf("plan: %+v", plan)
	}
	if !goal.Goal || goal.Rounds != DefaultRounds || goal.ReadOnly {
		t.Fatalf("goal: %+v", goal)
	}
	if build.ReadOnly || build.Goal || build.Default == "" {
		t.Fatalf("build: %+v", build)
	}
	for _, m := range list {
		if !strings.HasPrefix(Block(m), "Mode: "+m.Name+".") {
			t.Errorf("%s: the text must start with its name: %q", m.Name, Block(m))
		}
	}
}

func TestParse(t *testing.T) {
	list := Load()
	cases := []struct {
		in, name, rest string
		ok             bool
	}{
		{"#plan fix the login", "plan", "fix the login", true},
		{"  #PLAN\nfix it", "plan", "fix it", true},
		{"#build", "build", "", true},
		{"#goal ship v1 #plan", "goal", "ship v1 #plan", true},
		{"fix the #plan", "", "fix the #plan", false},
		{"#plans", "", "#plans", false},
		{"#plan-b x", "", "#plan-b x", false},
		{"#plan. x", "", "#plan. x", false},
		{"#unknown x", "", "#unknown x", false},
		{"# heading", "", "# heading", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		m, rest, ok := Parse(c.in, list)
		if ok != c.ok || m.Name != c.name || rest != c.rest {
			t.Errorf("Parse(%q) = %q, %q, %v; want %q, %q, %v", c.in, m.Name, rest, ok, c.name, c.rest, c.ok)
		}
	}
}

func TestLoadUserModes(t *testing.T) {
	global, project := t.TempDir(), t.TempDir()
	write := func(dir, name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(global, "review.md", "---\ndescription: read the diff\nreadonly: true\neffort: high\nnext: build\n---\nReview the changes.")
	write(global, "plan.md", "---\ndescription: my plan\n---\nPlan in three lines.")
	write(project, "review.md", "Project review.")
	write(project, "sprint.md", "---\ngoal: yes\nrounds: 3\n---\nFinish the sprint.")
	write(project, "empty.md", "---\ndescription: nothing\n---\n")
	write(project, "README.md", "not a mode")
	list := Load(global, project)
	names := map[string]Mode{}
	for _, m := range list {
		names[m.Name] = m
	}
	if _, ok := names["empty"]; ok {
		t.Fatal("a mode without text is no mode")
	}
	if _, ok := names["readme"]; ok {
		t.Fatal("README.md is not a mode")
	}
	if r := names["review"]; r.ReadOnly || r.Text != "Mode: review.\nProject review." {
		t.Fatalf("the project file wins and has its own header: %+v", r)
	}
	if p := names["plan"]; p.Builtin || p.Text != "Mode: plan.\nPlan in three lines." {
		t.Fatalf("a user file replaces a builtin: %+v", p)
	}
	if s := names["sprint"]; !s.Goal || s.Rounds != 3 {
		t.Fatalf("%+v", s)
	}
	write(global, "ro.md", "---\nreadonly: true\neffort: high\nnext: build\ndefault: look\n---\nLook only.")
	ro, _ := Find(Load(global), "ro")
	if !ro.ReadOnly || ro.Effort != "high" || ro.Next != "build" || ro.Default != "look" {
		t.Fatalf("%+v", ro)
	}
}
