package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDisableModelInvocationMarksSkillManualOnly(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name+".md")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	manual, _ := readSkill(write("a", "---\nname: a\ndescription: d\ndisable-model-invocation: true\n---\nbody"), "a", "global")
	normal, _ := readSkill(write("b", "---\nname: b\ndescription: d\n---\nbody"), "b", "global")
	other, _ := readSkill(write("c", "---\nname: c\ndisable-model-invocation: false\n---\nbody"), "c", "global")
	if !manual.ManualOnly || normal.ManualOnly || other.ManualOnly {
		t.Fatalf("manual=%v normal=%v other=%v", manual.ManualOnly, normal.ManualOnly, other.ManualOnly)
	}
}
