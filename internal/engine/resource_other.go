package engine

import (
	"encoding/json"
	"fmt"
	"meno/internal/agent"
	"meno/internal/attach"
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
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	img, err := attach.Prepare(raw)
	if err != nil {
		return nil, api(proto.CodeBadRequest, "view: "+err.Error())
	}
	if s.queue.Viewed() >= attach.MaxImages {
		return nil, api(proto.CodeConflict, "view: at most "+fmt.Sprint(attach.MaxImages)+" images per step")
	}
	if err = (attach.Store{Dir: e.paths.File("attachments")}).Put(img); err != nil {
		return nil, err
	}
	s.queue.Push(agent.Message{
		Text:        "Image from view(\"" + path + "\"): " + agent.ImageToken(1),
		Attachments: []agent.Attachment{{N: 1, Mime: img.Mime, SHA: img.SHA, W: img.W, H: img.H}},
		Viewed:      true,
	})
	return map[string]any{"path": abs, "displayed": true}, nil
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
	for _, p := range e.providerList() {
		if p.Disabled || provider != "" && provider != p.Name {
			continue
		}
		for id := range p.Models {
			out = append(out, map[string]any{"provider": p.Name, "id": id})
		}
	}
	return out
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
