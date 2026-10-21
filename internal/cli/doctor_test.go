package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runDoctorCLI(t *testing.T, f *fakeAPI, args ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(append([]string{"meno", "doctor"}, args...), IO{Out: &out, Err: &errb}, f)
	return code, out.String()
}

func TestDoctorExitCodesFollowWorstCheck(t *testing.T) {
	for _, tc := range []struct {
		status string
		code   int
	}{{"ok", 0}, {"warn", 4}, {"error", 1}} {
		f := &fakeAPI{report: DoctorReport{Checks: []Check{{Name: "x", Status: "ok"}, {Name: "y", Status: tc.status, Fix: "do z"}}}}
		code, out := runDoctorCLI(t, f)
		if code != tc.code {
			t.Errorf("%s: exit %d, want %d", tc.status, code, tc.code)
		}
		if tc.status != "ok" && !strings.Contains(out, "fix: do z") {
			t.Errorf("%s: no fix in output:\n%s", tc.status, out)
		}
	}
}

func TestDoctorJSONOutput(t *testing.T) {
	f := &fakeAPI{report: DoctorReport{Checks: []Check{{Name: "a", Status: "warn"}}}}
	_, out := runDoctorCLI(t, f, "--json")
	if !strings.Contains(out, `"status":"warn"`) || !strings.Contains(out, `"name":"a"`) {
		t.Fatalf("json: %s", out)
	}
}
