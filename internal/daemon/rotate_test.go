package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotateLogKeepsThreeFiles(t *testing.T) {
	p := filepath.Join(t.TempDir(), "daemon.log")
	for _, suffix := range []string{"", ".1", ".2", ".3"} {
		if e := os.WriteFile(p+suffix, make([]byte, maxLogBytes), 0o600); e != nil {
			t.Fatal(e)
		}
	}
	if e := RotateLog(p); e != nil {
		t.Fatal(e)
	}
	for _, suffix := range []string{"", ".1", ".2", ".3"} {
		if _, e := os.Stat(p + suffix); e != nil {
			t.Fatalf("missing %s: %v", suffix, e)
		}
	}
	info, e := os.Stat(p)
	if e != nil || info.Size() != 0 {
		t.Fatalf("new log %v %v", info, e)
	}
}
