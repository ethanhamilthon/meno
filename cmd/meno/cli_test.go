package main

import (
	"bytes"
	"strings"
	"testing"
)

func runCLI(args ...string) (code int, out, errOut string) {
	var o, e bytes.Buffer
	code = cli(args, &o, &e)
	return code, o.String(), e.String()
}

func TestVersionAndHelp(t *testing.T) {
	for _, flag := range []string{"-v", "--version", "version"} {
		if code, out, _ := runCLI(flag); code != 0 || out != "meno "+version+"\n" {
			t.Fatalf("%s: %d %q", flag, code, out)
		}
	}
	for _, flag := range []string{"-h", "--help", "help"} {
		if code, out, _ := runCLI(flag); code != 0 || !strings.Contains(out, "meno update") {
			t.Fatalf("%s: %d %q", flag, code, out)
		}
	}
}

func TestUnknownCommandFails(t *testing.T) {
	code, out, errOut := runCLI("load")
	if code != 1 || out != "" || !strings.Contains(errOut, `unknown command "load"`) || !strings.Contains(errOut, "meno -h") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
	if code, _, errOut := runCLI("update", "now"); code != 1 || !strings.Contains(errOut, "meno -h") {
		t.Fatalf("update with arguments: %d %q", code, errOut)
	}
}
