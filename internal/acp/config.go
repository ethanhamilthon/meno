package acp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"meno/internal/api"
)

func itoa(n int) string { return strconv.Itoa(n) }

func (s *Server) loadModels() {
	s.mu.Lock()
	have := s.models != nil
	s.mu.Unlock()
	if have {
		return
	}
	res, err := s.backend.Models(s.ctx, api.ModelsRequest{})
	if err != nil {
		return
	}
	sort.SliceStable(res.Models, func(i, j int) bool {
		if res.Models[i].Provider != res.Models[j].Provider {
			return res.Models[i].Provider < res.Models[j].Provider
		}
		return res.Models[i].ID < res.Models[j].ID
	})
	s.mu.Lock()
	s.models = res.Models
	s.mu.Unlock()
}

func (s *Server) findModel(provider, model string) *api.ConfigModel {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.models {
		if s.models[i].Provider == provider && s.models[i].ID == model {
			return &s.models[i]
		}
	}
	return nil
}

// contextWindow is the context size of a model, 0 when unknown.
func (s *Server) contextWindow(provider, model string) int {
	if m := s.findModel(provider, model); m != nil && m.ContextWindow != nil {
		return *m.ContextWindow
	}
	return 0
}

// configOptions describes the model and effort selectors of a session.
func (s *Server) configOptions(x *session) []map[string]any {
	x.mu.Lock()
	prov, model, effort := x.prov, x.model, x.effort
	x.mu.Unlock()
	s.mu.Lock()
	models := append([]api.ConfigModel(nil), s.models...)
	s.mu.Unlock()
	var opts []map[string]any
	cur := modelValue(prov, model)
	var list []map[string]any
	found := false
	for _, m := range models {
		v := modelValue(m.Provider, m.ID)
		found = found || v == cur
		list = append(list, map[string]any{"value": v, "name": m.Provider + " · " + m.ID})
	}
	if !found && model != "" {
		list = append(list, map[string]any{"value": cur, "name": prov + " · " + model})
	}
	if len(list) > 0 && model != "" {
		opts = append(opts, map[string]any{"id": "model", "name": "Model", "category": "model", "type": "select", "currentValue": cur, "options": list})
	}
	if m := s.findModel(prov, model); m != nil && len(m.Efforts) > 0 {
		var el []map[string]any
		have := false
		for _, e := range m.Efforts {
			have = have || e == effort
			el = append(el, map[string]any{"value": e, "name": e})
		}
		if !have {
			effort = m.Efforts[0]
		}
		opts = append(opts, map[string]any{"id": "effort", "name": "Reasoning effort", "category": "thought_level", "type": "select", "currentValue": effort, "options": el})
	}
	return opts
}

// sessionResult is the common part of the session/new and session/load results.
func (s *Server) sessionResult(x *session) map[string]any {
	out := map[string]any{"configOptions": s.configOptions(x)}
	x.mu.Lock()
	prov, model := x.prov, x.model
	x.mu.Unlock()
	if model != "" {
		var am []map[string]any
		s.mu.Lock()
		for _, m := range s.models {
			am = append(am, map[string]any{"modelId": modelValue(m.Provider, m.ID), "name": m.Provider + " · " + m.ID})
		}
		s.mu.Unlock()
		out["models"] = map[string]any{"currentModelId": modelValue(prov, model), "availableModels": am}
	}
	return out
}

func (s *Server) sendConfig(x *session) {
	x.send(map[string]any{"sessionUpdate": "config_option_update", "configOptions": s.configOptions(x)})
}

// setConfig changes the model or effort of an idle session as one validated step.
func (s *Server) setConfig(raw json.RawMessage) (any, error) {
	var p struct {
		SessionID string `json:"sessionId"`
		ConfigID  string `json:"configId"`
		Value     string `json:"value"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errParams("invalid params: %v", err)
	}
	x, err := s.session(p.SessionID)
	if err != nil {
		return nil, err
	}
	x.mu.Lock()
	busy := x.running || x.waiter != nil
	prov, model, effort := x.prov, x.model, x.effort
	x.mu.Unlock()
	if busy {
		return nil, errParams("the session is running; change the model when it is idle")
	}
	switch p.ConfigID {
	case "model":
		np, nm, ok := parseModelValue(p.Value)
		m := s.findModel(np, nm)
		if !ok || m == nil {
			return nil, errParams("unknown model %q", p.Value)
		}
		ne := effort
		if !contains(m.Efforts, ne) {
			ne = ""
		}
		prov, model, effort = np, nm, ne
	case "effort":
		m := s.findModel(prov, model)
		if m == nil || !contains(m.Efforts, p.Value) {
			return nil, errParams("effort %q is not supported by the current model", p.Value)
		}
		effort = p.Value
	default:
		return nil, errParams("unknown config option %q", p.ConfigID)
	}
	if _, err := s.backend.PatchSession(s.ctx, x.id, api.PatchSessionRequest{Provider: &prov, Model: &model, Effort: &effort}); err != nil {
		return nil, err
	}
	x.mu.Lock()
	x.prov, x.model, x.effort = prov, model, effort
	x.ctxSize = s.contextWindow(prov, model)
	x.mu.Unlock()
	return map[string]any{"configOptions": s.configOptions(x)}, nil
}

func contains(l []string, v string) bool {
	for _, e := range l {
		if e == v {
			return true
		}
	}
	return false
}

// commandList is the slash command metadata offered to the client.
func (s *Server) commandList() []map[string]any {
	cmds := []map[string]any{
		{"name": "answer", "description": "Answer an open question", "input": map[string]any{"hint": "<question-id> <answers-json> | <question-id> --cancel"}},
		{"name": "compact", "description": "Compact the conversation history"},
		{"name": "rename", "description": "Rename the session", "input": map[string]any{"hint": "<title>"}},
		{"name": "skills", "description": "List the available skills"},
		{"name": "context", "description": "Show the context usage"},
		{"name": "help", "description": "List the commands"},
	}
	if res, err := s.backend.Skills(s.ctx, ""); err == nil {
		for _, sk := range res.Skills {
			if sk.Enabled {
				cmds = append(cmds, map[string]any{"name": "skill:" + sk.Name, "description": sk.Desc, "input": map[string]any{"hint": "optional text"}})
			}
		}
	}
	return cmds
}

func (s *Server) sendCommands(x *session) {
	x.send(map[string]any{"sessionUpdate": "available_commands_update", "availableCommands": s.commandList()})
}

func (s *Server) skillsText() string {
	res, err := s.backend.Skills(s.ctx, "")
	if err != nil {
		return "Skills are unavailable."
	}
	var b strings.Builder
	for _, sk := range res.Skills {
		if sk.Enabled {
			fmt.Fprintf(&b, "- /skill:%s — %s\n", sk.Name, sk.Desc)
		}
	}
	if b.Len() == 0 {
		return "No skills are enabled."
	}
	return "Skills:\n" + b.String()
}

func (s *Server) contextText(x *session) string {
	snap, err := s.backend.Snapshot(s.ctx, x.id)
	if err != nil {
		return "Context is unavailable: " + err.Error()
	}
	text := fmt.Sprintf("Model: %s · %s\nMessages: %d\nContext tokens: %d", snap.Provider, snap.Model, snap.TurnCount, snap.Context)
	if w := s.contextWindow(snap.Provider, snap.Model); w > 0 {
		text += fmt.Sprintf(" of %d", w)
	}
	if !snap.Unpriced && snap.Spent > 0 {
		text += fmt.Sprintf("\nSpent: $%.4f", snap.Spent)
	}
	return text + "\nUpdated: " + time.Now().Format(time.RFC3339)
}
