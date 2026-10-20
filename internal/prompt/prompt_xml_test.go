package prompt

import (
	"strings"
	"testing"
)

func TestNoHeadingLinesAndEveryElementOpensOnce(t *testing.T) {
	{
		in := full()
		out := Build(in)
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "## ") {
				t.Fatalf("heading line %q", l)
			}
		}
		for _, name := range []string{"voice", "method", "guidelines", "skills", "global-instructions", "project-instructions", "mcp-servers", "environment"} {
			if strings.Count(out, "<"+name+">") != 1 || strings.Count(out, "</"+name+">") != 1 {
				t.Fatalf("<%s> must open and close once:\n%s", name, out)
			}
		}
		for _, name := range []string{"editing", "verification", "output"} {
			if strings.Count(out, "<"+name+">") != 1 || strings.Count(out, "</"+name+">") != 1 {
				t.Fatalf("<%s> must open and close once in an unrestricted prompt:\n%s", name, out)
			}
		}
	}
}
