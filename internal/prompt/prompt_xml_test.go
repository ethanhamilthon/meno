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
		for _, name := range []string{"skills", "project-instructions", "mcp-servers", "environment"} {
			if strings.Count(out, "<"+name+">") != 1 || strings.Count(out, "</"+name+">") != 1 {
				t.Fatalf("<%s> must open and close once:\n%s", name, out)
			}
		}
	}
}
