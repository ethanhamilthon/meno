package version

import (
	"regexp"
	"testing"
)

func TestNumberIsSemver(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(Number()) {
		t.Fatalf("VERSION %q is not MAJOR.MINOR.PATCH", Number())
	}
	if Label() != "v"+Number() {
		t.Fatalf("Label = %q", Label())
	}
}
