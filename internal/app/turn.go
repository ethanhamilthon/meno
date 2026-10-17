package app

import (
	"context"
	"strings"
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
)

func (a *App) submit() {
	if a.st.Ask != nil {
		a.answerTyped()
		return
	}
	text := strings.TrimSpace(string(a.st.Input))
	if text == "" || a.st.Running {
		return
	}
	if _, ok := a.currentProvider(); !ok {
		a.notifyErr("provider not configured · select provider and model with :model")
		return
	}
	a.setInput(nil)
	a.st.Undo, a.st.Redo, a.st.InsBase = nil, nil, a.snapshot()
	a.startTurn(text)
}

func (a *App) startTurn(text string) {
	a.reloadSkills()
	p, ok := a.currentProvider()
	if !ok {
		a.notifyErr("provider not configured for model " + a.st.Settings.Model)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()
	a.st.Running = true
	a.st.Phase = "thinking"
	a.st.Scroll = 0
	a.clearLive()
	sess := a.st.Session
	sess.Append(agent.Turn{Role: llm.RoleUser, Text: text})
	runner := &agent.Runner{D: agent.Deps{
		Client: a.client(p), Provider: p, Model: a.st.Settings.Model,
		Effort: a.st.Settings.Effort, System: a.systemPrompt(),
		VM: a.vm, Thinking: a.st.Settings.Thinking != "off", ModelTimeout: a.st.Settings.ModelTimeout(),
		Emit:  func(e agent.Ev) { a.postEvent(agentEv{ev: e}) },
		Retry: a.retry,
	}}
	a.running.Add(1)
	go func() {
		defer a.running.Done()
		defer cancel()
		err := runner.Continue(ctx, sess)
		a.postEvent(turnDone{err: err})
	}()
}

func (a *App) abortTurn() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *App) onAgent(e agent.Ev) {
	if e.Kind != agent.EvRetry && e.Kind != agent.EvUsage {
		a.st.Retry = nil
	}
	switch e.Kind {
	case agent.EvRetry:
		// The failed request's partial output is void: the request starts over.
		a.clearLive()
		a.st.Phase = "retrying"
		a.st.Retry = &retryState{Attempt: e.Attempt, Max: e.Max, Until: time.Now().Add(e.Delay), Reason: briefReason(e.Err)}
		a.notify("retry " + itoa(e.Attempt) + "/" + itoa(e.Max) + " in " + fmtWait(e.Delay) + " — " + a.st.Retry.Reason)
		return
	}
	switch e.Kind {
	case agent.EvThink, agent.EvText, agent.EvCode, agent.EvJSStart, agent.EvJSEnd:
		a.st.LiveBase = e.Turns
	}
	switch e.Kind {
	case agent.EvThink:
		a.st.LiveThink += e.Thinking
		a.st.Phase = "thinking"
	case agent.EvText:
		a.st.LiveText += e.Text
		a.st.Phase = "writing"
	case agent.EvCode:
		a.st.Phase = "writing"
		a.liveCall(e.CallID).Code = e.Code
	case agent.EvJSStart:
		lc := a.liveCall(e.CallID)
		lc.Code, lc.Running = e.Code, true
		a.st.Phase = "running"
	case agent.EvJSEnd:
		// The finished call (and the text before it) is in the session now.
		a.dropLiveCall(e.CallID)
		a.st.LiveText, a.st.LiveThink = "", ""
		a.st.Phase = "thinking"
	case agent.EvDone, agent.EvAborted:
		a.clearLive()
		a.st.Phase = ""
	case agent.EvUsage:
		u := e.Usage
		a.st.Usage.In += u.In
		a.st.Usage.Out += u.Out
		a.st.Usage.Cached += u.Cached
		a.st.Usage.CacheWrite += u.CacheWrite
		a.st.LastUsage = u
		a.st.Context = u.In + u.Out
		p, model := e.Provider, e.Model
		if model == "" {
			p, _ = a.currentProvider()
			model = a.st.Settings.Model
		}
		record := agent.RequestUsage{Provider: p.Name, Model: model, Usage: u, Time: time.Now().UTC()}
		if info, ok := llm.LookupMetadata(a.st.Pricing, p, model); ok && info.CanPrice(u) {
			cost := info.Cost(u)
			record.Cost = &cost
			a.st.Spent += cost
		} else if u.In > 0 || u.Out > 0 || u.Cached > 0 || u.CacheWrite > 0 {
			a.st.Unpriced = true
		}
		if a.st.Session != nil {
			a.st.Session.Requests = append(a.st.Session.Requests, record)
		}
	case agent.EvError:
		a.notifyErr(e.Err)
	}
}

func (a *App) onTurnDone(err error) {
	a.st.Running = false
	a.st.Retry = nil
	a.st.Phase = ""
	a.mu.Lock()
	a.cancel = nil
	a.mu.Unlock()
	a.clearLive()
	if err != nil && !isAbort(err) {
		a.notifyErr(err.Error())
	}
	a.st.Session.Metrics = agent.Metrics{LastUsage: a.st.LastUsage, Usage: a.st.Usage, Spent: a.st.Spent, Context: a.st.Context, Unpriced: a.st.Unpriced}
	if err := agent.SaveSession(a.paths.Home, a.st.Session); err != nil {
		a.notifyErr("save failed: " + err.Error())
	}
}

func isAbort(err error) bool {
	return err == context.Canceled
}

func (a *App) currentProvider() (llm.Provider, bool) {
	if a.st.Settings.Model == "" || a.st.Settings.Provider == "" {
		return llm.Provider{}, false
	}
	for _, p := range a.st.Providers {
		if p.Name == a.st.Settings.Provider {
			return p, true
		}
	}
	return llm.Provider{}, false
}

func (a *App) scrollBy(d int) {
	w, h := a.scr.Size()
	maxScroll := max(0, len(a.transcriptWithSelection(w))-max(0, h-a.bottomHeight()))
	a.st.Scroll = min(max(0, a.st.Scroll+d), maxScroll)
}

func (a *App) clearLive() {
	a.st.LiveText, a.st.LiveThink, a.st.LiveCalls = "", "", nil
}

// liveCall finds the live call with this id. Chat streams may deliver code
// before the id is known: such a call is adopted by the first event that has
// one.
func (a *App) liveCall(id string) *liveCall {
	calls := a.st.LiveCalls
	for i := range calls {
		if id != "" && calls[i].ID == id {
			return &calls[i]
		}
	}
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].ID == "" && !calls[i].Running {
			calls[i].ID = id
			return &calls[i]
		}
		break
	}
	a.st.LiveCalls = append(a.st.LiveCalls, liveCall{ID: id})
	return &a.st.LiveCalls[len(a.st.LiveCalls)-1]
}

func (a *App) dropLiveCall(id string) {
	out := a.st.LiveCalls[:0]
	for _, c := range a.st.LiveCalls {
		if c.ID != id {
			out = append(out, c)
		}
	}
	a.st.LiveCalls = out
}

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
