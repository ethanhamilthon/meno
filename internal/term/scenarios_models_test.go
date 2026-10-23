//go:build termtest

package term

import (
	"testing"
)

// `meno models` in an agent call lists what the provider reports on /models,
// although the provider has no explicit model settings.
func TestAgentModelsListsDiscoveredModels(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		if q.Mentions("fake/other-model") {
			q.Text("models-listed")
		} else {
			q.Tool("", "meno models --provider fake")
		}
	}
	s := start(t, options{fake: f})
	s.Say("list models")
	s.WaitFor("models-listed")
}
