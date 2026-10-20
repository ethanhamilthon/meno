package daemonapi

import (
	"encoding/json"
	"net/url"

	"meno/internal/cli"
)

// Ask puts a form to the human and waits for the answers.
func (a *API) Ask(form []byte) (map[string]any, error) {
	var out map[string]any
	return out, a.post("ask", "", map[string]any{"form": json.RawMessage(form)}, &out)
}

// GoalDone ends a goal run with a summary.
func (a *API) GoalDone(summary string) error {
	return a.post("goal", "done", map[string]any{"summary": summary}, nil)
}

// GoalBlocked ends a goal run that cannot go on.
func (a *API) GoalBlocked(reason string) error {
	return a.post("goal", "blocked", map[string]any{"reason": reason}, nil)
}

// View asks the client to show an image after the call.
func (a *API) View(path string) (cli.ViewResult, error) {
	var out cli.ViewResult
	return out, a.post("view", "", map[string]any{"path": path}, &out)
}

// Models lists the models of the enabled providers.
func (a *API) Models(provider string, refresh bool) ([]cli.Model, error) {
	var out []cli.Model
	q := url.Values{}
	if provider != "" {
		q.Set("provider", provider)
	}
	if refresh {
		q.Set("refresh", "true")
	}
	return out, a.get("models", "", q.Encode(), &out)
}
