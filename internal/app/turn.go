package app

import (
	"context"
	"strings"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
)

func (a *App) submit() {
	text := strings.TrimSpace(string(a.st.Input))
	if text == "" || a.st.Running {
		return
	}
	if strings.HasPrefix(text, "#") {
		if p, ok := configFindPrompt(a.st.Prompts, text); ok {
			a.st.Input = []rune(p.Template)
			return
		}
		a.st.Toast = "unknown prompt " + text
		return
	}
	a.st.Input = nil
	a.startTurn(text)
}

func (a *App) startTurn(text string) {
	p, ok := a.currentProvider()
	if !ok {
		a.st.Toast = "provider not configured for model " + a.st.Settings.Model
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()
	a.st.Running = true
	sess := a.st.Session
	runner := &agent.Runner{D: agent.Deps{
		Client: a.client(p), Provider: p, Model: a.st.Settings.Model,
		Effort: a.st.Settings.Effort, System: systemPrompt(a.paths.Cwd),
		VM: a.vm, Thinking: a.st.Settings.Thinking == "on",
		Emit: func(e agent.Ev) { a.events <- agentEv{ev: e} },
	}}
	a.running.Add(1)
	go func() {
		defer a.running.Done()
		err := runner.Turn(ctx, sess, text)
		a.events <- turnDone{err: err}
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
	switch e.Kind {
	case agent.EvUsage:
		a.st.Usage = e.Usage
	case agent.EvError:
		a.st.Toast = e.Err
	}
}

func (a *App) onTurnDone(err error) {
	a.st.Running = false
	a.cancel = nil
	if err != nil && !isAbort(err) {
		a.st.Toast = err.Error()
	}
	_ = agent.SaveSession(a.paths.SessionsDir(), a.st.Session)
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

func systemPrompt(cwd string) string {
	return "You are meno, a coding agent. Your only tool is js: run JavaScript with fs.read/write/edit/glob/exists and sh(cmd). Working directory: " + cwd + "."
}

func (a *App) lastTurnIndex() int {
	turns := a.st.Session.Snapshot()
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Code != "" {
			return i
		}
	}
	return 0
}

func (a *App) nextJSTurn(from, step int) int {
	turns := a.st.Session.Snapshot()
	for i := from + step; i >= 0 && i < len(turns); i += step {
		if turns[i].Code != "" {
			return i
		}
	}
	return from
}

func (a *App) selectedCode() string {
	turns := a.st.Session.Snapshot()
	if a.st.Visual < 0 || a.st.Visual >= len(turns) {
		return ""
	}
	return turns[a.st.Visual].Code
}

func (a *App) runSelectedJS() {
	code := a.selectedCode()
	if code == "" {
		return
	}
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "[direct run]", Code: code})
	res := a.vm.Exec(context.Background(), code)
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Code: code, Output: res.Output + res.Value, Err: res.Err, CallID: agent.NewID()})
}

func (a *App) runLastJS() {
	a.st.Mode = ui.ModeVisual
	a.st.Visual = a.lastTurnIndex()
	a.runSelectedJS()
	a.st.Mode = ui.ModeNormal
}

func (a *App) scrollBy(d int) {
	a.st.Scroll += d
	if a.st.Scroll < 0 {
		a.st.Scroll = 0
	}
}
