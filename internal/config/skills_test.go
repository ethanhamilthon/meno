package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func skillStore(t *testing.T) (*Store, Paths) {
	t.Helper()
	p := Paths{Home: filepath.Join(t.TempDir(), "home"), Cwd: filepath.Join(t.TempDir(), "proj")}
	return NewStore(p), p
}

func TestNoSkillsOnFreshInstall(t *testing.T) {
	st, _ := skillStore(t)
	sk, err := st.LoadSkills()
	if err != nil || len(sk) != 0 {
		t.Fatalf("placeholder skills are back: %v %v", sk, err)
	}
}

func TestDiscoverSkills(t *testing.T) {
	st, p := skillStore(t)
	g, pr := p.GlobalSkillsDir(), p.ProjectSkillsDir()
	write(t, filepath.Join(g, "Docker Deploy.md"), "# Docker\n\nShip the image with docker.\n\nMore text.\n")
	write(t, filepath.Join(g, "shared.md"), "---\nname: shared\ndescription: global version\n---\nbody\n")
	write(t, filepath.Join(pr, "shared.md"), "---\ndescription: \"project version\"\n---\nbody\n")
	write(t, filepath.Join(pr, "api-security", "SKILL.md"), "---\nname: API Security\ndescription: check auth,\n  rate limits\n---\n# body\n")
	write(t, filepath.Join(pr, "README.md"), "not a skill")
	write(t, filepath.Join(pr, "notes.txt"), "not a skill")
	write(t, filepath.Join(pr, "empty-dir", "other.md"), "not a skill")
	sk, err := st.LoadSkills()
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ Name, Scope, Desc string }
	var got []row
	for _, s := range sk {
		if !s.Enabled || s.Path == "" {
			t.Fatalf("skill %+v must be enabled and have a path", s)
		}
		got = append(got, row{s.Name, s.Scope, s.Desc})
	}
	want := []row{
		{"api-security", "project", "check auth, rate limits"},
		{"docker-deploy", "global", "Ship the image with docker."},
		{"shared", "project", "project version"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestSkillSwitchesPersist(t *testing.T) {
	st, p := skillStore(t)
	write(t, filepath.Join(p.GlobalSkillsDir(), "a.md"), "first\n")
	write(t, filepath.Join(p.GlobalSkillsDir(), "b.md"), "second\n")
	sk, _ := st.LoadSkills()
	sk[0].Enabled = false
	if err := st.SaveSkills(sk); err != nil {
		t.Fatal(err)
	}
	again, _ := st.LoadSkills()
	if again[0].Enabled || !again[1].Enabled {
		t.Fatalf("state lost: %+v", again)
	}
	// A skill that vanishes and returns keeps its switch.
	if err := os.Remove(again[0].Path); err != nil {
		t.Fatal(err)
	}
	rest, _ := st.LoadSkills()
	if err := st.SaveSkills(rest); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(p.GlobalSkillsDir(), "a.md"), "first\n")
	back, _ := st.LoadSkills()
	if back[0].Name != "a" || back[0].Enabled {
		t.Fatalf("switch forgotten: %+v", back)
	}
}

func TestOldSkillsFileIsIgnored(t *testing.T) {
	st, p := skillStore(t)
	write(t, p.File("skills.json"), `[{"name":"api-security","scope":"project","desc":"x","enabled":true}]`)
	write(t, filepath.Join(p.GlobalSkillsDir(), "real.md"), "text\n")
	sk, err := st.LoadSkills()
	if err != nil || len(sk) != 1 || sk[0].Name != "real" || !sk[0].Enabled {
		t.Fatalf("%+v %v", sk, err)
	}
}

func TestFilterSkills(t *testing.T) {
	sk := []Skill{{Name: "api-security", Desc: "auth"}, {Name: "docker", Desc: "deploy"}}
	if len(FilterSkills(sk, "api")) != 1 || len(FilterSkills(sk, "DEPLOY")) != 1 || len(FilterSkills(sk, "")) != 2 {
		t.Fatal("filter")
	}
}
