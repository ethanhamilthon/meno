package app

import (
	"strings"
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
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

// pricer prices one request of the model, for the cost in the run summary.
func (a *App) pricer(p llm.Provider, model string) func(llm.Usage) (float64, bool) {
	info, ok := llm.LookupMetadata(a.st.Pricing, p, model)
	if !ok {
		return nil
	}
	return func(u llm.Usage) (float64, bool) {
		if !info.CanPrice(u) {
			return 0, false
		}
		return info.Cost(u), true
	}
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

// runningText is the status-bar text while the agent works: which request of
// the run it is on, for how long, how many js calls and the tokens spent.
func (a *App) runningText() string {
	if !a.st.Running || a.st.RunStart.IsZero() {
		return ""
	}
	turn := a.st.RunTurns + 1 // the request being answered
	if a.st.Phase == "running" {
		turn = max(1, a.st.RunTurns) // the call belongs to the request just answered
	}
	parts := []string{"turn " + itoa(turn), clock(time.Since(a.st.RunStart))}
	if n := a.st.RunJS; n > 0 {
		if n == 1 {
			parts = append(parts, "1 js call")
		} else {
			parts = append(parts, itoa(n)+" js calls")
		}
	}
	in, out := a.st.Usage.In-a.st.RunUsage0.In, a.st.Usage.Out-a.st.RunUsage0.Out
	if in > 0 || out > 0 {
		parts = append(parts, "↓ "+compactTokens(max(0, in))+" ↑ "+compactTokens(max(0, out)))
	}
	return strings.Join(parts, " · ")
}

// autoCompact is the safety net settings for the model of this turn. An unknown
// context window turns it off: a window is never guessed.
func (a *App) autoCompact(p llm.Provider, model string) agent.AutoCompact {
	s := a.st.Settings
	ac := agent.AutoCompact{Percent: s.CompactPercent, Keep: s.CompactKeep, SummaryTokens: s.CompactSummaryTokens}
	if info, ok := llm.LookupMetadata(a.st.Pricing, p, model); ok {
		ac.Window = info.Context
	}
	return ac
}
