package engine

import (
	"errors"
	"fmt"
	"strings"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/llm"
)

// modelChoice is the provider, model and effort a session ends up with.
type modelChoice struct{ provider, model, effort string }

// pickModel applies an optional PROVIDER/MODEL and effort on top of cur and
// checks the result: the provider must be configured and enabled, a model the
// provider lists must exist, and the effort must be one the model supports. A
// new model keeps the current effort when it accepts it, else the closest one.
func (e *Engine) pickModel(spec, effort string, cur modelChoice) (modelChoice, error) {
	out := cur
	if spec != "" {
		p, m, err := e.parseModelSpec(spec, cur.provider)
		if err != nil {
			return cur, err
		}
		out.provider, out.model = p, m
	}
	if spec == "" && effort == "" {
		return out, nil
	}
	p, ok := e.provider(out.provider)
	if !ok {
		return cur, dapi(api.CodeBadRequest, fmt.Sprintf("provider %q is not configured or is disabled", out.provider))
	}
	allowed := p.EffortsFor(out.model, config.Choices("effort"))
	if effort != "" {
		if !contains(allowed, effort) {
			return cur, dapi(api.CodeBadRequest, fmt.Sprintf("effort %q is not supported by %s/%s (allowed: %s)", effort, out.provider, out.model, strings.Join(allowed, ", ")))
		}
		out.effort = effort
	} else {
		out.effort = llm.ClampEffort(allowed, out.effort)
	}
	return out, nil
}

// parseModelSpec reads PROVIDER/MODEL. The provider is the part before the
// first slash, so model ids that contain slashes work. A spec without a known
// provider is a model id of the default provider.
func (e *Engine) parseModelSpec(spec, defProvider string) (string, string, error) {
	provider, model := defProvider, spec
	if name, rest, ok := strings.Cut(spec, "/"); ok {
		if _, known := e.provider(name); known {
			provider, model = name, rest
		}
	}
	p, ok := e.provider(provider)
	if !ok {
		return "", "", dapi(api.CodeBadRequest, fmt.Sprintf("unknown model %q: use PROVIDER/MODEL as listed by `meno models`", spec))
	}
	if model == "" {
		return "", "", dapi(api.CodeBadRequest, fmt.Sprintf("model %q has no model id: use PROVIDER/MODEL", spec))
	}
	if _, listed := p.Models[model]; len(p.Models) > 0 && !listed {
		return "", "", dapi(api.CodeBadRequest, fmt.Sprintf("provider %s has no model %q: see `meno models --provider %s`", provider, model, provider))
	}
	return provider, model, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// systemPrompt resolves the custom system prompt of a new session. An explicit
// name must exist ("default" is the built-in prompt). Without one, the
// system_prompt setting applies; when its file is gone the built-in prompt is
// used and `meno doctor` reports it. The text is frozen in the session.
func (e *Engine) systemPrompt(explicit string) (name, text string, err error) {
	if explicit == config.DefaultSystemPrompt {
		return "", "", nil
	}
	if explicit == "" {
		name = e.cfg().SystemPrompt
		if name == "" {
			return "", "", nil
		}
		if text, err = e.paths.LoadSystemPrompt(name); err != nil {
			return "", "", nil
		}
		return name, text, nil
	}
	if text, err = e.paths.LoadSystemPrompt(explicit); err != nil {
		code := api.CodeBadRequest
		if errors.Is(err, config.ErrSystemPromptMissing) {
			code = api.CodeNotFound
		}
		return "", "", dapi(code, err.Error())
	}
	return explicit, text, nil
}
