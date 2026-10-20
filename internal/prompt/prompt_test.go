package prompt

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

func full() Input {
	return Input{
		Cwd:           "/w",
		Skills:        []Skill{{"b", "second", "/p/b.md"}, {"a", "first", "/p/a.md"}},
		GlobalAgents:  Instructions{"/h/AGENTS.md", "global text\n"},
		ProjectAgents: Instructions{"/w/AGENTS.md", "project text"},
		Tools:         []Tool{{"s2", "x", "", "{}"}, {"s1", "y", "", "{}"}, {"s1", "a", "", "{}"}},
		Now:           now,
		OS:            "macOS 15.1 (darwin/arm64)",
		Shell:         "/bin/sh",
	}
}

func TestSectionOrder(t *testing.T) {
	out := Build(full())
	if out != Build(full()) {
		t.Fatal("identical input must produce byte-identical prompts")
	}
	last := -1
	for _, h := range []string{
		"You are meno, a coding agent.", "<voice>", "<method>", "<editing>", "<verification>", "<output>", "<guidelines>", "<skills>",
		"<global-instructions>", "<project-instructions>", "<mcp-servers>", "<environment>",
	} {
		i := strings.Index(out, h)
		if i <= last {
			t.Fatalf("%q out of order (at %d, previous %d):\n%s", h, i, last, out)
		}
		last = i
	}
	if !strings.HasPrefix(out, "You are meno, a coding agent.\n\n<voice>") {
		t.Fatal("identity must be a bare first line")
	}
	if !strings.HasSuffix(out, "- Working directory: /w\n- OS: macOS 15.1 (darwin/arm64). Shell: /bin/sh.\n- Date: 2026-10-17, Saturday (UTC).\n</environment>") {
		t.Fatalf("environment must be last:\n%s", out)
	}
	if strings.Index(out, "- a —") > strings.Index(out, "- b —") || !strings.Contains(out, "- s1 (2 tools): a, y") || strings.Index(out, "- s1 (") > strings.Index(out, "- s2 (") {
		t.Fatalf("lists must be sorted:\n%s", out)
	}
}

func TestEnvironmentNamesTheModel(t *testing.T) {
	in := full()
	in.Provider, in.Model, in.Effort = "openai", "gpt-5.5", "high"
	out := Build(in)
	if !strings.HasSuffix(out, "- Date: 2026-10-17, Saturday (UTC).\n- Model: gpt-5.5 (openai, effort high).\n</environment>") {
		t.Fatalf("model line must close the environment:\n%s", out)
	}
	if got := DescribeModel("", "m", ""); got != "m" {
		t.Fatal(got)
	}
	if got := DescribeModel("p", "m", ""); got != "m (p)" {
		t.Fatal(got)
	}
}

func TestOptionalSectionsAbsentWhenEmpty(t *testing.T) {
	out := Build(Input{Cwd: "/w", Now: now})
	for _, h := range []string{"skills", "global-instructions", "project-instructions", "mcp-servers"} {
		if strings.Contains(out, "<"+h+">") || strings.Contains(out, "</"+h+">") {
			t.Fatalf("%s must be absent:\n%s", h, out)
		}
	}
	if !strings.Contains(out, "<environment>") || !strings.Contains(out, "<guidelines>") {
		t.Fatal("bare prompt lost a fixed section")
	}
}

func TestInstructionsFormat(t *testing.T) {
	out := Build(full())
	want := "<global-instructions>\nFrom /h/AGENTS.md. Follow these unless the user's message says otherwise.\n\nglobal text\n</global-instructions>\n\n<project-instructions>\nFrom /w/AGENTS.md. Follow these unless the user's message says otherwise.\n\nproject text\n</project-instructions>\n\n"
	if !strings.Contains(out, want) {
		t.Fatalf("instructions format:\n%s", out)
	}
}

// Changing a section leaves everything above it byte-identical.
func TestPrefixIsByteStable(t *testing.T) {
	baseOut := Build(full())
	cases := []struct {
		name   string
		mod    func(*Input)
		marker string // first heading that may differ
	}{
		{"skills", func(i *Input) { i.Skills = append(i.Skills, Skill{"c", "", "/c.md"}) }, "<skills>"},
		{"global agents", func(i *Input) { i.GlobalAgents.Text = "other" }, "<global-instructions>"},
		{"project agents", func(i *Input) { i.ProjectAgents = Instructions{} }, "<project-instructions>"},
		{"tools", func(i *Input) { i.Tools = i.Tools[:1] }, "<mcp-servers>"},
		{"cwd and date", func(i *Input) { i.Cwd = "/other"; i.Now = now.AddDate(0, 0, 1) }, "<environment>"},
	}
	for _, c := range cases {
		in := full()
		c.mod(&in)
		out := Build(in)
		if out == baseOut {
			t.Fatalf("%s: no change", c.name)
		}
		cut := strings.Index(baseOut, c.marker)
		if cut < 0 || !strings.HasPrefix(out, baseOut[:cut]) {
			t.Fatalf("%s: prefix before %q changed", c.name, c.marker)
		}
	}
}

func TestBuildSkillsAndTools(t *testing.T) {
	out := Build(Input{Cwd: "/w",
		Skills: []Skill{{"api-security", "check\nauth  and rate limits", "/p/.meno/skills/api-security.md"}, {"bare", "", "/p/bare.md"}},
		Tools:  []Tool{{"github", "search-repos", "Search repositories", "{query: string, page?: number}"}}})
	for _, want := range []string{
		"<skills>", "- api-security — check auth and rate limits (/p/.meno/skills/api-security.md)", "- bare (/p/bare.md)",
		"<mcp-servers>", "- github (1 tool): search_repos",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestBuildCapsLongToolLists(t *testing.T) {
	var tools []Tool
	for i := 0; i < maxListedTools+7; i++ {
		tools = append(tools, Tool{Server: "s", Name: "t" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Params: "{}"})
	}
	out := Build(Input{Cwd: "/w", Tools: tools})
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "- s (") {
			line = l
		}
	}
	if strings.Count(line, ", ") != maxListedTools || !strings.Contains(line, "and 7 more") || !strings.Contains(line, "(67 tools)") {
		t.Fatalf("not capped: %q", line)
	}
}

func TestLoadInstructions(t *testing.T) {
	dir := t.TempDir()
	if in, err := LoadInstructions(filepath.Join(dir, "none.md")); err != nil || in != (Instructions{}) {
		t.Fatalf("missing file: %+v %v", in, err)
	}
	empty := filepath.Join(dir, "empty.md")
	os.WriteFile(empty, []byte(" \n"), 0o644)
	if in, err := LoadInstructions(empty); err != nil || in != (Instructions{}) {
		t.Fatalf("empty file: %+v %v", in, err)
	}
	if _, err := LoadInstructions(dir); err == nil {
		t.Fatal("a directory must be an error")
	}
	big := filepath.Join(dir, "big.md")
	os.WriteFile(big, []byte(strings.Repeat("я", MaxInstructions)), 0o644) // 2 bytes per rune
	in, err := LoadInstructions(big)
	if err != nil || !strings.Contains(in.Text, "[truncated: ") || len(in.Text) > MaxInstructions+64 || !strings.HasPrefix(in.Text, "яяя") {
		t.Fatalf("truncation: err=%v len=%d tail=%q", err, len(in.Text), in.Text[max(0, len(in.Text)-40):])
	}
	if strings.Contains(in.Text, "\ufffd") {
		t.Fatal("cut in the middle of a rune")
	}
	if in.Path != big {
		t.Fatal("path")
	}
}

func TestDescribeOSAndDate(t *testing.T) {
	if !strings.Contains(DescribeOS(), "/") {
		t.Fatal(DescribeOS())
	}
	if got := FormatDate(now); got != "2026-10-17, Saturday (UTC)" {
		t.Fatal(got)
	}
}

func TestDefaultJSBuildMatchesStoredBytes(t *testing.T) {
	in := Input{Cwd: "/w", Now: now, OS: "macOS 15.1 (darwin/arm64)", Shell: "/bin/sh"}
	const want = "5698023a708c222520a67f7141906039e2159490adb3397d79498a6435460034"
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(Build(in)))); got != want {
		t.Fatalf("default js prompt changed: %s", got)
	}
}

func TestIdent(t *testing.T) {
	for in, want := range map[string]string{"github": "github", "my-server": "my_server", "9lives": "_lives", "a.b": "a_b", "": "_"} {
		if got := Ident(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
