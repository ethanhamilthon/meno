package engine

import (
	"encoding/json"
	"fmt"
	"meno/internal/proto"
	"net/url"
	"os"
	"path/filepath"
)

func (e *Engine) viewResource(s *runState, data map[string]any) (any, error) {
	path := stringValue(data, "path")
	if path == "" {
		return nil, api(proto.CodeBadRequest, "view path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if _, err = os.Stat(abs); err != nil {
		return nil, err
	}
	return map[string]any{"path": abs, "displayed": false}, nil
}
func (e *Engine) goalResource(s *runState, tail []string, data map[string]any) (any, error) {
	if s.goal == nil {
		return nil, api(proto.CodeConflict, "goal is not active")
	}
	op := "done"
	if len(tail) > 0 {
		op = tail[0]
	}
	text := stringValue(data, "summary")
	if op == "blocked" {
		text = stringValue(data, "reason")
		return nil, s.goal.Blocked(text)
	}
	return nil, s.goal.Done(text)
}
func (e *Engine) modelResource(query string) any {
	v, _ := url.ParseQuery(query)
	provider := v.Get("provider")
	out := []map[string]any{}
	for _, p := range e.providers {
		if p.Disabled || provider != "" && provider != p.Name {
			continue
		}
		for id := range p.Models {
			out = append(out, map[string]any{"provider": p.Name, "id": id})
		}
	}
	return out
}
func (e *Engine) askResource(s *runState, data map[string]any) (any, error) {
	return nil, api(proto.CodeBadRequest, "ask is only available as a paused tool call")
}
func stringValue(m map[string]any, k string) string { v, _ := m[k].(string); return v }
func boolValue(m map[string]any, k string, def bool) bool {
	v, ok := m[k].(bool)
	if !ok {
		return def
	}
	return v
}
func number(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}
func mapEnv(v any) []string {
	m, _ := v.(map[string]any)
	var out []string
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s=%v", k, v))
	}
	return out
}
func envSlice(m map[string]string) []string {
	var out []string
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}
