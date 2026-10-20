package engine

import (
	"context"
	"errors"
	"meno/internal/agent"
	"meno/internal/attach"
	"meno/internal/websearch"
	"os"
	"path/filepath"
	"strings"
)

type toolHost struct {
	engine *Engine
	state  *runState
}

func (h toolHost) Models(opts map[string]any) (any, error) {
	var out []map[string]any
	for _, p := range h.engine.providerList() {
		if p.Disabled {
			continue
		}
		for id := range p.Models {
			v := map[string]any{"provider": p.Name, "id": id}
			out = append(out, v)
		}
	}
	return out, nil
}

func (h toolHost) Search(ctx context.Context, q string, o map[string]any) (any, error) {
	count := 5
	if n, ok := o["count"].(int64); ok {
		count = int(n)
	}
	backend := h.engine.cfg().WebSearch
	if backend == "" || backend == "auto" || backend == "native" {
		backend = "auto"
	}
	var errs []string
	do := func(b websearch.Backend, key string) (any, bool) {
		if key == "" && b != websearch.DuckDuckGo {
			return nil, false
		}
		ans, err := websearch.Search(ctx, b, key, q, count)
		if err != nil {
			errs = append(errs, err.Error())
			return nil, false
		}
		res := make([]map[string]string, 0, len(ans.Results))
		for _, r := range ans.Results {
			res = append(res, map[string]string{"title": r.Title, "url": r.URL, "snippet": r.Snippet})
		}
		return map[string]any{"query": q, "backend": string(b), "answer": ans.Answer, "results": res}, len(res) > 0 || ans.Answer != ""
	}
	for _, b := range []websearch.Backend{websearch.Brave, websearch.Tavily} {
		if backend == "auto" || backend == string(b) {
			if v, ok := do(b, h.engine.cfg().WebSearchKey); ok {
				return v, nil
			}
		}
	}
	if v, ok := do(websearch.DuckDuckGo, ""); ok {
		return v, nil
	}
	return nil, errors.New(strings.Join(errs, "; "))
}
func (h toolHost) View(display string, img attach.Image) (int, error) {
	s := h.state
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 1
	for _, m := range s.queue.Items() {
		n += len(m.Attachments)
	}
	path := filepath.Join(h.engine.paths.File("attachments"), img.SHA+attach.Ext(img.Mime))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return 0, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.WriteFile(path, img.Data, 0600); err != nil {
			return 0, err
		}
	}
	s.queue.Push(agent.Message{Text: "The agent asked to view " + display, Attachments: []agent.Attachment{{N: n, Mime: img.Mime, SHA: img.SHA, W: img.W, H: img.H}}, Viewed: true})
	return n, nil
}
