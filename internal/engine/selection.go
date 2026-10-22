package engine

import (
	"fmt"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/llm"
)

// errNoModel is what a session with nothing usable to run gets.
const msgNoModel = "No model selected. Choose a provider and model with /model."

// selectionInput is one consistent view of the defaults and the providers. It
// is taken before a session lock is held, because the engine lock is never
// taken inside a session lock.
type selectionInput struct {
	settings  config.Settings
	providers []llm.Provider
}

func (e *Engine) selectionInput() selectionInput {
	return selectionInput{settings: e.cfg(), providers: e.providerList()}
}

func (in selectionInput) provider(name string) (llm.Provider, bool) {
	for _, p := range in.providers {
		if p.Name == name {
			return p, true
		}
	}
	return llm.Provider{}, false
}

// resolveSelection makes sure the session can start a run. The caller holds
// s.mu and the session is idle. It returns true when it changed the session.
//
//   - Both provider and model empty: take provider, model and effort from the
//     current defaults. Defaults must name an enabled provider; the model need
//     not be in the discovery cache, custom endpoints accept unlisted models.
//   - Only one of the two set: the user must choose, nothing is guessed.
//   - A complete pair: kept as it is; a removed or disabled provider is
//     reported by name, never replaced by the defaults.
func resolveSelection(s *runState, in selectionInput) (bool, error) {
	sess := s.session
	switch {
	case sess.Provider == "" && sess.Model == "":
		d := in.settings
		if d.Provider == "" || d.Model == "" {
			return false, dapi(api.CodeBadRequest, msgNoModel)
		}
		p, ok := in.provider(d.Provider)
		if !ok || p.Disabled {
			return false, dapi(api.CodeBadRequest, msgNoModel)
		}
		sess.Provider, sess.Model, sess.Effort = d.Provider, d.Model, d.Effort
		return true, nil
	case sess.Provider == "" || sess.Model == "":
		return false, dapi(api.CodeBadRequest, msgNoModel)
	}
	p, ok := in.provider(sess.Provider)
	if !ok {
		return false, dapi(api.CodeBadRequest, fmt.Sprintf("Provider %q is not configured. Choose another with /model.", sess.Provider))
	}
	if p.Disabled {
		return false, dapi(api.CodeBadRequest, fmt.Sprintf("Provider %q is disabled. Enable it in /login or choose another with /model.", sess.Provider))
	}
	return false, nil
}
