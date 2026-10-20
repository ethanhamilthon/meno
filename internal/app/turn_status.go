package app

import (
	"strings"
	"time"
)

// briefReason squeezes a provider error into one short line.
func briefReason(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 70 {
		s = string(r[:69]) + "…"
	}
	return s
}

// fmtWait renders a wait as whole seconds, rounded up.
func fmtWait(d time.Duration) string {
	return itoa(int((d+time.Second-1)/time.Second)) + "s"
}

// retryText is the status-bar text while a retry is pending, with a countdown.
func (a *App) retryText() string {
	r := a.st.Retry
	if r == nil {
		return ""
	}
	head := "retry " + itoa(r.Attempt) + "/" + itoa(r.Max)
	if left := time.Until(r.Until); left > 0 {
		head += " in " + fmtWait(left)
	} else {
		head += " · sending"
	}
	return head + " — " + r.Reason
}

// clock writes how long something has gone on: 0:42, 12:05, 1:02:03.
func clock(d time.Duration) string {
	s := int(max(0, d) / time.Second)
	if s >= 3600 {
		return itoa(s/3600) + ":" + pad2(s%3600/60) + ":" + pad2(s%60)
	}
	return itoa(s/60) + ":" + pad2(s%60)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}
