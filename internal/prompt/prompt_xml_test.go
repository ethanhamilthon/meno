package prompt

import (
	"strings"
	"testing"
)

// The exact prompt of a fixed input. It guards the format: if "## " headings
// come back, or an element changes, this test fails.
func TestBuildGolden(t *testing.T) {
	in := full()
	in.Provider, in.Model, in.Effort = "openai", "gpt-5.5", "high"
	want := `You are meno, a coding agent.

<guidelines>
- Your only tool is js. Its description lists the globals you can use.
- Batch independent work in one js call. Use tasks or agents for long parallel work.
- Explore first. Implement only when you have enough information.
- Give short final answers. Do not explain details if the user did not ask.
- Ask: use askUser({questions}). It shows all questions in one form. See help("askUser").
</guidelines>

<skills>
A skill is an instruction file for one type of work. When the task matches a skill, read its file with fs.read before you start. Obey the file. If the user writes /name, read that skill first.
- a — first (/p/a.md)
- b — second (/p/b.md)
</skills>

<global-instructions>
From /h/AGENTS.md. Follow these unless the user's message says otherwise.

global text
</global-instructions>

<project-instructions>
From /w/AGENTS.md. Follow these unless the user's message says otherwise.

project text
</project-instructions>

<mcp-servers>
- s1 (2 tools): a, y
- s2 (1 tool): x
</mcp-servers>

<environment>
- Working directory: /w
- OS: macOS 15.1 (darwin/arm64). Shell: /bin/sh.
- Date: 2026-10-17, Saturday (UTC).
- Model: gpt-5.5 (openai, effort high).
</environment>`
	if got := Build(in); got != want {
		t.Fatalf("prompt changed:\n%s", got)
	}
}

func TestSubagentPromptHasRoleAndNoAskRule(t *testing.T) {
	in := full()
	in.Subagent = true
	out := Build(in)
	if !strings.Contains(out, "<subagent-role>\nA parent agent started you") || !strings.Contains(out, "</subagent-role>") {
		t.Fatalf("no subagent role:\n%s", out)
	}
	if strings.Contains(out, "askUser") {
		t.Fatalf("a subagent has no ask rule:\n%s", out)
	}
	if strings.Index(out, "</guidelines>") > strings.Index(out, "<subagent-role>") ||
		strings.Index(out, "</subagent-role>") > strings.Index(out, "<skills>") {
		t.Fatalf("the role goes between guidelines and skills:\n%s", out)
	}
}

func TestNoHeadingLinesAndEveryElementOpensOnce(t *testing.T) {
	for _, sub := range []bool{false, true} {
		in := full()
		in.Subagent = sub
		out := Build(in)
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "## ") {
				t.Fatalf("heading line %q (subagent=%v)", l, sub)
			}
		}
		for _, name := range []string{"guidelines", "skills", "global-instructions", "project-instructions", "mcp-servers", "environment"} {
			if strings.Count(out, "<"+name+">") != 1 || strings.Count(out, "</"+name+">") != 1 {
				t.Fatalf("<%s> must open and close once (subagent=%v):\n%s", name, sub, out)
			}
		}
	}
}
