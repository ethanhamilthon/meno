//go:build termtest

package term

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// meno -p prints the final answer on stdout, resumes a stored session, and an
// unknown command that was removed says what replaces it.
func TestHeadlessPrintAndResume(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		if q.Mentions("second question") {
			q.Text("second answer")
			return
		}
		q.Text("first answer")
	}
	home, work := t.TempDir(), t.TempDir()
	seedHome(t, options{fake: f, home: home}, work)
	run := func(args ...string) (string, string, error) {
		cmd := exec.Command(binPath, args...)
		cmd.Dir = work
		cmd.Env = append(cmd.Environ(), "MENO_HOME="+home, "HOME="+home, "MENO_NO_METADATA=1", "MENO_NO_UPDATE_CHECK=1")
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		err := cmd.Run()
		return out.String(), errb.String(), err
	}
	out, errOut, err := run("-p", "first question")
	if err != nil || strings.TrimSpace(out) != "first answer" {
		t.Fatalf("print: %q %q %v", out, errOut, err)
	}
	out, errOut, err = run("-p", "--continue", "second question")
	if err != nil || strings.TrimSpace(out) != "second answer" {
		t.Fatalf("continue: %q %q %v", out, errOut, err)
	}
	if _, errOut, err = run("sessions"); err == nil || !strings.Contains(errOut, "meno -p") {
		t.Fatalf("a removed command names its replacement: %q %v", errOut, err)
	}
}
