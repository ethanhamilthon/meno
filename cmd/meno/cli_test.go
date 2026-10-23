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
		if code, out, _ := runCLI(flag); code != 0 || !strings.Contains(out, "meno <command>") {
			t.Fatalf("%s: %d %q", flag, code, out)
		}
	}
}

func TestUnknownCommandFails(t *testing.T) {
	code, out, errOut := runCLI("load")
	if code != 1 || out != "" || !strings.Contains(errOut, `unknown command "load"`) || !strings.Contains(errOut, "Run 'meno -h'") {
		t.Fatalf("%d %q %q", code, out, errOut)
	}
	if code, _, errOut := runCLI("update", "now"); code != 1 || !strings.Contains(errOut, "meno -h") {
		t.Fatalf("update with arguments: %d %q", code, errOut)
	}
	// meno web was removed in 0.6.9: it is an ordinary unknown command.
	if code, out, errOut := runCLI("web", "search", "x"); code != 1 || out != "" || !strings.Contains(errOut, `unknown command "web"`) {
		t.Fatalf("meno web: %d %q %q", code, out, errOut)
	}
	if _, out, _ := runCLI("-h"); strings.Contains(out, "web") {
		t.Fatalf("meno -h still mentions web: %q", out)
	}
}

// Every command the cli package implements must be routed to it by the entry
// point, or the real binary answers "unknown command".
func TestEveryCLICommandIsRoutedToTheDaemonGroup(t *testing.T) {
	for _, name := range []string{"mcp", "tasks", "ask", "view", "models", "sessions", "send", "tail", "doctor"} {
		if !isDaemonGroup(name) {
			t.Errorf("%q is not routed to the cli package", name)
		}
	}
}

// --no-ask is accepted by `meno` alone and by `meno acp`; nothing else is.
func TestNoAskFlagIsNarrow(t *testing.T) {
	code, _, errOut := runCLI("--no-ask", "send", "x")
	if code != 1 || !strings.Contains(errOut, `unknown command "--no-ask"`) {
		t.Fatalf("meno --no-ask send: %d %q", code, errOut)
	}
	var o, e bytes.Buffer
	if code := runACP([]string{"--bogus"}, strings.NewReader(""), &o, &e); code != 2 || !strings.Contains(e.String(), "acp takes only --no-ask") {
		t.Fatalf("acp --bogus: %d %q", code, e.String())
	}
}
