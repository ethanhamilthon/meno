package cli

import (
	"reflect"
	"strings"
	"testing"
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
	code, _, errs := run([]string{"send", "--", "-s", "literal text"}, a, "")
	if code != 0 || errs != "" || len(a.requests) != 1 || a.requests[0].Text != "-s literal text" || a.requests[0].SessionID != "" {
		t.Fatalf("code=%d errors=%q requests=%+v", code, errs, a.requests)
	}
}
