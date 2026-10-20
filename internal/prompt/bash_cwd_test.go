package prompt

import (
	"strings"
	"testing"
)

func TestBashWorkingDirectoryGuidelines(t *testing.T) {
	for _, groups := range [][]string{nil, {"help"}} {
		got := bashGuidelines(groups)
		for _, want := range []string{
			"working directory shown in the environment section",
			"Use relative paths; do not prefix commands with cd to that directory.",
			"Use cd only when a command needs a different directory.",
			"Directory changes do not persist between calls",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("groups %v: missing %q", groups, want)
			}
		}
	}
}
