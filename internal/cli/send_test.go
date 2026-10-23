package cli

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

type sendRecorder struct {
	fakeAPI
	requests []SendRequest
}

func (a *sendRecorder) Send(r SendRequest) (SendResult, error) {
	a.requests = append(a.requests, r)
	return SendResult{SessionID: r.SessionID}, nil
}

func TestSendSessionFlag(t *testing.T) {
	for _, flag := range []string{"-s", "--s"} {
		t.Run(flag, func(t *testing.T) {
			a := &sendRecorder{}
			code, _, errs := run([]string{"send", flag, "target-session", "--detach", "stand by"}, a, "")
			want := []SendRequest{{SessionID: "target-session", Text: "stand by", Detach: true}}
			if code != 0 || errs != "" || !reflect.DeepEqual(a.requests, want) {
				t.Fatalf("code=%d errors=%q requests=%+v", code, errs, a.requests)
			}
		})
	}
}

func TestSendShortSessionFlagValidation(t *testing.T) {
	for _, tc := range []struct {
		args []string
		hint string
	}{
		{[]string{"send", "-s"}, "needs a value"},
		{[]string{"send", "-s", "target-session"}, "usage:"},
		{[]string{"send", "-s", "target-session", "--new", "hello"}, "cannot be combined"},
	} {
		a := &sendRecorder{}
		code, _, errs := run(tc.args, a, "")
		if code != ExitUsage || !strings.Contains(errs, tc.hint) || len(a.requests) != 0 {
			t.Errorf("%v: code=%d errors=%q requests=%+v", tc.args, code, errs, a.requests)
		}
	}
}

func TestSendLiteralSessionFlag(t *testing.T) {
	a := &sendRecorder{}
	code, _, errs := run([]string{"send", "--new", "--", "-s", "literal text"}, a, "")
	if code != 0 || errs != "" || len(a.requests) != 1 || a.requests[0].Text != "-s literal text" || a.requests[0].SessionID != "" || !a.requests[0].New {
		t.Fatalf("code=%d errors=%q requests=%+v", code, errs, a.requests)
	}
}

func TestSendNoAskNeedsANewSession(t *testing.T) {
	a := &sendRecorder{}
	code, _, errs := run([]string{"send", "--new", "--no-ask", "--detach", "hi"}, a, "")
	want := []SendRequest{{Text: "hi", New: true, NoAsk: true, Detach: true}}
	if code != 0 || errs != "" || !reflect.DeepEqual(a.requests, want) {
		t.Fatalf("code=%d errors=%q requests=%+v", code, errs, a.requests)
	}
	for _, args := range [][]string{
		{"send", "-s", "abc", "--no-ask", "hi"},
	} {
		b := &sendRecorder{}
		code, _, errs := run(args, b, "")
		if code != 2 || !strings.Contains(errs, "--no-ask applies to a new session") || len(b.requests) != 0 {
			t.Fatalf("%v: code=%d errors=%q requests=%+v", args, code, errs, b.requests)
		}
	}
}

// There is no implicit session: a send that does not say where to write is a usage error.
func TestSendNeedsASessionChoice(t *testing.T) {
	a := &sendRecorder{}
	code, _, errs := run([]string{"send", "hello"}, a, "")
	if code != ExitUsage || !strings.Contains(errs, "choose where to write") || len(a.requests) != 0 {
		t.Fatalf("code=%d errors=%q requests=%+v", code, errs, a.requests)
	}
}

func TestSendForkAndOptions(t *testing.T) {
	a := &sendRecorder{}
	code, _, errs := run([]string{"send", "--fork", "src", "--at", "3", "--model", "p/m", "--effort", "low", "--cwd", "/w", "--title", "T", "--system", "writer", "--no-ask", "--timeout", "30", "--keep-running", "go", "on"}, a, "")
	want := []SendRequest{{Text: "go on", Fork: "src", At: 3, Model: "p/m", Effort: "low", CWD: "/w", Title: "T", System: "writer", NoAsk: true, Timeout: 30 * time.Second, KeepRunning: true}}
	if code != 0 || errs != "" || !reflect.DeepEqual(a.requests, want) {
		t.Fatalf("code=%d errors=%q requests=%+v", code, errs, a.requests)
	}
}

func TestSendRejectsInconsistentFlags(t *testing.T) {
	for args, hint := range map[string]string{
		"send --fork a --new hi":                       "cannot be combined",
		"send -s a --fork b hi":                        "cannot be combined",
		"send --fork a --at 0 hi":                      "--at must be an integer",
		"send --new --at 2 hi":                         "--at applies to --fork",
		"send -s a --cwd /x hi":                        "--cwd applies to a new session or a fork",
		"send -s a --title T hi":                       "--title applies to a new session or a fork",
		"send -s a --system s hi":                      "--system applies to a new session or a fork",
		"send --new --detach --timeout 5 hi":           "cannot be combined with --detach",
		"send --new --timeout 0 hi":                    "--timeout must be an integer",
		"send --new --keep-running --cancel-on-exit x": "cannot be combined",
	} {
		a := &sendRecorder{}
		code, _, errs := run(strings.Fields(args), a, "")
		if code != ExitUsage || !strings.Contains(errs, hint) || len(a.requests) != 0 {
			t.Errorf("%q: code=%d errors=%q requests=%+v", args, code, errs, a.requests)
		}
	}
}

type resultAPI struct {
	fakeAPI
	res SendResult
	err error
}

func (a *resultAPI) Send(SendRequest) (SendResult, error) { return a.res, a.err }

func TestSendExitCodes(t *testing.T) {
	waiting := SendResult{SessionID: "s1", Status: "waiting", Text: "so far", Question: &PendingQuestion{ID: "a1", Questions: []QuestionItem{{ID: "q1", Prompt: "ok?", Type: "text"}}}}
	code, out, errs := run([]string{"send", "-s", "s1", "hi"}, &resultAPI{res: waiting}, "")
	if code != ExitNeedsAnswer || out != "so far" || !strings.Contains(errs, "meno sessions answer s1 a1") {
		t.Fatalf("waiting: code=%d out=%q errors=%q", code, out, errs)
	}
	code, out, _ = run([]string{"send", "-s", "s1", "--json", "hi"}, &resultAPI{res: waiting}, "")
	if code != ExitNeedsAnswer || !strings.Contains(out, `"status":"waiting"`) || !strings.Contains(out, `"id":"a1"`) {
		t.Fatalf("waiting json: code=%d out=%q", code, out)
	}
	aborted := &resultAPI{res: SendResult{SessionID: "s1", Status: "aborted"}, err: errAborted}
	code, out, errs = run([]string{"send", "-s", "s1", "--json", "hi"}, aborted, "")
	if code != ExitAborted || !strings.Contains(out, `"status":"aborted"`) || !strings.Contains(errs, "stopped") {
		t.Fatalf("aborted: code=%d out=%q errors=%q", code, out, errs)
	}
	timeout := &resultAPI{res: SendResult{SessionID: "s1", Status: "timeout"}, err: &APIError{Code: "wait_timeout", Message: "slow"}}
	if code, _, _ = run([]string{"send", "-s", "s1", "--timeout", "1", "hi"}, timeout, ""); code != ExitWaitTimeout {
		t.Fatalf("timeout: %d", code)
	}
	queued := &resultAPI{res: SendResult{SessionID: "s1", Status: "queued", Queued: true}}
	if code, out, errs = run([]string{"send", "-s", "s1", "hi"}, queued, ""); code != 0 || out != "" || !strings.Contains(errs, "queued") {
		t.Fatalf("queued: code=%d out=%q errors=%q", code, out, errs)
	}
	detached := &resultAPI{res: SendResult{SessionID: "s9", Status: "detached"}}
	if code, out, _ = run([]string{"send", "--new", "--detach", "hi"}, detached, ""); code != 0 || out != "s9\n" {
		t.Fatalf("detached new: code=%d out=%q", code, out)
	}
}
