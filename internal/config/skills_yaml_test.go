package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func readFixture(t *testing.T, dir, text string) Skill {
	t.Helper()
	p := filepath.Join(t.TempDir(), dir, "SKILL.md")
	write(t, p, text)
	s, ok := readSkill(p, dir, "global")
	if !ok {
		t.Fatal("skill not loaded")
	}
	return s
}

func TestYAMLFoldedAndLiteralDescriptions(t *testing.T) {
	s := readFixture(t, "example", "---\nname: example\ndescription: >\n  First line\n  second line.\n---\nbody\n")
	if s.Desc != "First line second line." {
		t.Fatalf("folded: %q", s.Desc)
	}
	s = readFixture(t, "example", "---\nname: example\ndescription: |\n  Line one\n  Line two\n---\nbody\n")
	if s.Desc != "Line one Line two" {
		t.Fatalf("literal: %q", s.Desc)
	}
}

func TestYAMLInlineCommentAndQuotes(t *testing.T) {
	s := readFixture(t, "example", "---\nname: example # skill identifier\ndescription: \"Say \\\"hi\\\"\\tnow\"\n---\nbody\n")
	if s.Name != "example" {
		t.Fatalf("name %q", s.Name)
	}
	if s.Desc != "Say \"hi\" now" {
		t.Fatalf("desc %q", s.Desc)
	}
	s = readFixture(t, "example", "---\nname: 'example'\ndescription: 'it''s fine: really'\n---\n")
	if s.Desc != "it's fine: really" {
		t.Fatalf("single quoted %q", s.Desc)
	}
}

func TestYAMLNestedMetadataDoesNotLeak(t *testing.T) {
	s := readFixture(t, "example", "---\nname: example\ndescription: Does things\nmetadata:\n  author: me\n  version: \"1.0\"\nallowed-tools: Bash(git:*) Read\n---\nbody\n")
	if s.Desc != "Does things" || s.Name != "example" {
		t.Fatalf("%+v", s)
	}
	meta, _ := ParseFrontmatter("---\nname: x\nmetadata:\n  author: me\n---\n")
	if _, ok := meta["author"]; ok || meta["metadata"] != "" {
		t.Fatalf("nested leaked: %v", meta)
	}
}

func TestYAMLInvalidFallsBackLeniently(t *testing.T) {
	s := readFixture(t, "example", "---\nname: example\ndescription: Use when: the user asks\n---\nbody\n")
	if s.Desc != "Use when: the user asks" {
		t.Fatalf("desc %q", s.Desc)
	}
	if len(s.Warnings) == 0 || !strings.Contains(strings.Join(s.Warnings, ";"), "not valid YAML") {
		t.Fatalf("warnings %v", s.Warnings)
	}
}

func TestSkillDiagnostics(t *testing.T) {
	s := readFixture(t, "example", "---\nname: Other_Name\n---\n# T\n")
	j := strings.Join(s.Warnings, ";")
	for _, want := range []string{"missing description", "not a valid skill name", "does not match the directory"} {
		if !strings.Contains(j, want) {
			t.Fatalf("no %q in %v", want, s.Warnings)
		}
	}
	ok := readFixture(t, "example", "---\nname: example\ndescription: fine\n---\n")
	if len(ok.Warnings) != 0 {
		t.Fatalf("clean skill has warnings: %v", ok.Warnings)
	}
	if b := readFixture(t, "bare", "just text\n"); !strings.Contains(strings.Join(b.Warnings, ";"), "no frontmatter") {
		t.Fatalf("%v", b.Warnings)
	}
}

func TestAgentsSkillDirsPrecedence(t *testing.T) {
	root := t.TempDir()
	p := Paths{Home: filepath.Join(root, "u", ".meno"), Cwd: filepath.Join(root, "proj")}
	write(t, filepath.Join(p.UserAgentsSkillsDir(), "s", "SKILL.md"), "---\nname: s\ndescription: user agents\n---\n")
	write(t, filepath.Join(p.UserAgentsSkillsDir(), "only-user", "SKILL.md"), "---\nname: only-user\ndescription: u\n---\n")
	write(t, filepath.Join(p.GlobalSkillsDir(), "s", "SKILL.md"), "---\nname: s\ndescription: meno global\n---\n")
	write(t, filepath.Join(p.ProjectAgentsSkillsDir(), "s", "SKILL.md"), "---\nname: s\ndescription: project agents\n---\n")
	write(t, filepath.Join(p.ProjectAgentsSkillsDir(), "t", "SKILL.md"), "---\nname: t\ndescription: t agents\n---\n")
	write(t, filepath.Join(p.ProjectSkillsDir(), "t", "SKILL.md"), "---\nname: t\ndescription: t meno\n---\n")
	got := map[string]string{}
	for _, s := range DiscoverSkillDirs(p.SkillDirs()) {
		got[s.Name] = s.Desc + "/" + s.Scope
	}
	want := map[string]string{"s": "project agents/project", "only-user": "u/global", "t": "t meno/project"}
	if len(got) != 3 || got["s"] != want["s"] || got["only-user"] != want["only-user"] || got["t"] != want["t"] {
		t.Fatalf("got %v", got)
	}
}

func TestCustomHomeIgnoresUserAgentsDir(t *testing.T) {
	p := Paths{Home: filepath.Join(t.TempDir(), "custom"), Cwd: t.TempDir()}
	if p.UserAgentsSkillsDir() != "" {
		t.Fatal("custom home must not read ~/.agents")
	}
}
