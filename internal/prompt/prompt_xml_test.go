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

<voice>
Answer in the shortest form that is still complete.
- No preamble, no restating the request, no "let me", no offer to continue, no praise.
- Say what changed and what you checked, one line per fact. No headings.
- Bullets only for three or more items.
- Do not explain code you did not change.
- Say "unverified" instead of guessing.
</voice>

<method>
- Find before you change: fs.search, then read the part you need. Never guess a path or a symbol.
- Do independent work in one call: several reads, several edits, or a read and its check.
- Do not read a file again when nothing changed it. Its result is still above.
- Read back only the range you edited, not the whole file.
- Never wait by sleeping. Use tasks.wait for a background task.
- Stop when the task is done. No unrequested refactors, comments or files.
</method>

<editing>
- An existing file changes with fs.inspect and then fs.patch with expectedHash. Keep each oldText short and unique.
- fs.write creates a new file. It refuses an existing file on purpose; do not pass overwrite to replace one.
- Never send a whole file back to change part of it. That body is sent again in every later request and is the largest thing you write.
</editing>

<verification>
- Check the smallest thing that can fail: the package you touched, not the whole repo.
- Run the full suite once, before you say the work is done.
- A failing test is not done until it passes or until you say why it cannot.
- Do not run the same command twice unchanged.
</verification>

<output>
- Ask the program for less: head, tail, a filter, a narrower flag. Do not dump a directory or a whole log.
- Return the value you need, not the whole object.
- Everything a call prints stays in the context for the rest of the run and is paid for again in every request.
</output>

<guidelines>
- Your only tool is js. Its description lists the globals you can use.
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
	in.Only = []string{"agents", "goal", "askUser", "help", "hash", "path", "base64", "btoa", "atob"}
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
	if !strings.Contains(out, "<method>") {
		t.Fatalf("subagent policy must keep method rules:\n%s", out)
	}
	for _, name := range []string{"editing", "verification", "output"} {
		if strings.Contains(out, "<"+name+">") {
			t.Errorf("subagent policy must omit <%s>", name)
		}
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
		for _, name := range []string{"voice", "method", "guidelines", "skills", "global-instructions", "project-instructions", "mcp-servers", "environment"} {
			if strings.Count(out, "<"+name+">") != 1 || strings.Count(out, "</"+name+">") != 1 {
				t.Fatalf("<%s> must open and close once (subagent=%v):\n%s", name, sub, out)
			}
		}
		for _, name := range []string{"editing", "verification", "output"} {
			if strings.Count(out, "<"+name+">") != 1 || strings.Count(out, "</"+name+">") != 1 {
				t.Fatalf("<%s> must open and close once in an unrestricted prompt:\n%s", name, out)
			}
		}
	}
}
