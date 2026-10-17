package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

type responsesClient struct{}

func responsesInput(msgs []Msg) []map[string]any {
	var in []map[string]any
	for _, m := range msgs {
		if m.Role == RoleAssistant && len(m.Responses) != 0 {
			var items []map[string]any
			hasMessage := false
			calls := map[string]bool{}
			for _, raw := range m.Responses {
				var item map[string]any
				if json.Unmarshal(raw, &item) == nil {
					items = append(items, item)
					if item["type"] == "message" {
						hasMessage = true
					}
					if item["type"] == "function_call" {
						id, _ := item["call_id"].(string)
						calls[id] = true
					}
				}
			}
			// Some compatible streams omit message/item completion events.
			// Do not let available native reasoning or calls hide streamed text.
			if !hasMessage && m.Text != "" {
				in = append(in, map[string]any{"role": m.Role, "content": m.Text})
			}
			in = append(in, items...)
			for _, c := range m.Calls() {
				if !calls[c.ID] {
					in = append(in, map[string]any{"type": "function_call", "call_id": c.ID, "name": c.Name, "arguments": c.Arguments})
				}
			}
			continue
		}
		switch {
		case m.Role == RoleTool:
			in = append(in, map[string]any{"type": "function_call_output", "call_id": m.CallID, "output": m.Text})
		case m.Role == RoleAssistant && len(m.Calls()) != 0:
			if m.Text != "" {
				in = append(in, map[string]any{"role": m.Role, "content": m.Text})
			}
			for _, c := range m.Calls() {
				in = append(in, map[string]any{"type": "function_call", "call_id": c.ID, "name": c.Name, "arguments": c.Arguments})
			}
		default:
			in = append(in, map[string]any{"role": m.Role, "content": m.Text})
		}
	}
	return in
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (responsesClient) Stream(ctx context.Context, req Request, emit func(Event)) error {
	body := map[string]any{
		"model":        req.Model,
		"instructions": req.System,
		"input":        responsesInput(req.Msgs),
		"stream":       true,
		"include":      []string{"reasoning.encrypted_content"},
	}
	if req.Effort != "" {
		body["reasoning"] = map[string]any{"effort": req.Effort, "summary": "auto"}
	}
	if req.Tools {
		body["tools"] = []any{ToolSchema(req.ModelTimeout)}
	}
	applyCache(body, req)
	resp, err := post(ctx, req.Provider, "/responses", body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	state := responsesStream{items: map[string]any{}}
	var done Event
	err = ReadSSE(resp.Body, func(e SSEEvent) error {
		return state.handle(e, func(ev Event) {
			if ev.Kind == EvDone {
				done = ev
			} else {
				emit(ev)
			}
		})
	})
	if err != nil {
		return err
	}
	if !state.completed {
		return fmt.Errorf("responses stream: %w", io.ErrUnexpectedEOF)
	}
	emit(done)
	return nil
}

type respEv struct {
	Type     string    `json:"type"`
	Delta    string    `json:"delta"`
	ItemID   string    `json:"item_id"`
	Item     *respItem `json:"item"`
	Response *struct {
		Status string `json:"status"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
		IncompleteDetails *struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
		Usage *struct {
			Details struct {
				Cached     int `json:"cached_tokens"`
				CacheWrite int `json:"cache_write_tokens"`
			} `json:"input_tokens_details"`
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"response"`
	Message string `json:"message"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type respItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type responsesStream struct {
	completed bool
	items     map[string]any
	args      map[string]string // streamed arguments by item id
	calls     map[string]string // item id -> call id
}

func handleResponsesEvent(e SSEEvent, emit func(Event)) error {
	s := responsesStream{items: map[string]any{}}
	return s.handle(e, emit)
}

func (s *responsesStream) handle(e SSEEvent, emit func(Event)) error {
	if e.Data == "[DONE]" {
		return nil
	}
	var ev respEv
	if err := json.Unmarshal([]byte(e.Data), &ev); err != nil {
		return fmt.Errorf("responses stream: %w", err)
	}
	if ev.Type == "" {
		ev.Type = e.Event
	}
	if ev.Type == "error" || ev.Type == "response.failed" || ev.Type == "response.incomplete" || ev.Error != nil || (ev.Response != nil && (ev.Response.Error != nil || ev.Response.Status == "failed" || ev.Response.Status == "incomplete")) {
		message := ev.Message
		if ev.Error != nil {
			message = ev.Error.Message
		}
		if ev.Response != nil {
			if ev.Response.Error != nil {
				message = ev.Response.Error.Message
			} else if ev.Response.IncompleteDetails != nil {
				message = ev.Response.IncompleteDetails.Reason
			}
		}
		err := fmt.Errorf("responses stream %s: %s", ev.Type, message)
		emit(Event{Kind: EvError, Text: err.Error()})
		return err
	}
	if s.completed {
		return fmt.Errorf("responses event after completion: %s", ev.Type)
	}
	switch ev.Type {
	case "response.output_text.delta":
		emit(Event{Kind: EvText, Text: ev.Delta})
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		emit(Event{Kind: EvThinking, Text: ev.Delta})
	case "response.output_item.added":
		if ev.Item != nil && ev.Item.Type == "function_call" {
			if s.calls == nil {
				s.calls, s.args = map[string]string{}, map[string]string{}
			}
			s.calls[ev.Item.ID] = ev.Item.CallID
			emit(Event{Kind: EvToolDelta, CallID: ev.Item.CallID})
		}
	case "response.function_call_arguments.delta":
		if s.args == nil {
			s.calls, s.args = map[string]string{}, map[string]string{}
		}
		s.args[ev.ItemID] += ev.Delta
		emit(Event{Kind: EvToolDelta, CallID: s.calls[ev.ItemID], Code: PartialCode(s.args[ev.ItemID])})
	case "response.output_item.done":
		var raw struct {
			Item json.RawMessage `json:"item"`
		}
		_ = json.Unmarshal([]byte(e.Data), &raw)
		if ev.Item == nil {
			return fmt.Errorf("responses output_item.done missing item")
		}
		if ev.Item.Type == "function_call" && ev.Item.CallID == "" {
			return fmt.Errorf("responses function_call missing call_id")
		}
		var item any
		if err := json.Unmarshal(raw.Item, &item); err != nil {
			return err
		}
		var keys []string
		if ev.Item.ID != "" {
			keys = append(keys, "item:"+ev.Item.ID)
		}
		if ev.Item.Type == "function_call" {
			keys = append(keys, "call:"+ev.Item.CallID)
		}
		repeated := false
		for _, key := range keys {
			if previous, ok := s.items[key]; ok {
				if !reflect.DeepEqual(previous, item) {
					return fmt.Errorf("conflicting responses output item %s", key)
				}
				repeated = true
			}
		}
		for _, key := range keys {
			s.items[key] = item
		}
		if repeated {
			return nil
		}
		if ev.Item.Type == "function_call" {
			emit(Event{Kind: EvTool, CallID: ev.Item.CallID, Name: ev.Item.Name, Arguments: ev.Item.Arguments, Code: parseCode(ev.Item.Arguments), Item: raw.Item})
		} else if ev.Item != nil {
			emit(Event{Kind: EvHistory, Item: raw.Item})
		}
	case "response.completed":
		if ev.Response != nil && ev.Response.Status != "" && ev.Response.Status != "completed" {
			return fmt.Errorf("responses completion status: %s", ev.Response.Status)
		}
		s.completed = true
		u := Usage{}
		if ev.Response != nil && ev.Response.Usage != nil {
			u = Usage{In: ev.Response.Usage.InputTokens, Out: ev.Response.Usage.OutputTokens, Cached: ev.Response.Usage.Details.Cached, CacheWrite: ev.Response.Usage.Details.CacheWrite}
		}
		emit(Event{Kind: EvDone, Usage: u})
	}
	return nil
}

func parseCode(args string) string {
	var a struct {
		Code string `json:"code"`
	}
	if json.Unmarshal([]byte(args), &a) != nil {
		return args
	}
	return a.Code
}
