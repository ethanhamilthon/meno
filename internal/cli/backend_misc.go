package cli

import (
	"encoding/json"
	"net/url"
)

// Ask puts a form to the human and waits for the answers.
func (a *DaemonAPI) Ask(form []byte) (map[string]any, error) {
	var out map[string]any
	return out, a.post("ask", "", map[string]any{"form": json.RawMessage(form)}, &out)
}

// GoalDone ends a goal run with a summary.

// GoalBlocked ends a goal run that cannot go on.

// View asks the client to show an image after the call.
func (a *DaemonAPI) View(path string) (ViewResult, error) {
	var out ViewResult
	return out, a.post("view", "", map[string]any{"path": path}, &out)
}

// Models lists the models of the enabled providers.
func (a *DaemonAPI) Models(provider string, refresh bool) (ModelList, error) {
	var out ModelList
	q := url.Values{}
	if provider != "" {
		q.Set("provider", provider)
	}
	if refresh {
		q.Set("refresh", "true")
	}
	return out, a.get("models", "", q.Encode(), &out)
}
