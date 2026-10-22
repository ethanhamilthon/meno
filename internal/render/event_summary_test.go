package render

import "testing"

func TestSystemEventSummaryNamesTheEvent(t *testing.T) {
	for in, want := range map[string]string{
		"<system-event>plan-open: you are about to finish</system-event>":                                                           "system: plan-open",
		"<system-event>This is a message from the system itself.\n<date-changed>The date is now x.</date-changed>\n</system-event>": "system: date-changed",
		"<system-event><cwd-changed>moved</cwd-changed></system-event>":                                                             "system: cwd-changed",
	} {
		if got := eventSummary(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}
