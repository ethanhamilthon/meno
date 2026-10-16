package llm

import (
	"context"
	"encoding/json"
)

type responsesClient struct{}

func responsesInput(msgs []Msg) []map[string]any {
	var in []map[string]any
	for _, m := range msgs {
		switch {
		case m.Role == RoleTool:
			in = append(in, map[string]any{"type": "function_call_output", "call_id": m.CallID, "output": m.Text})
		case m.Role == RoleAssistant && m.Code != "":
			in = append(in, map[string]any{"type": "function_call", "call_id": m.CallID, "name": ToolName, "arguments": mustJSON(map[string]string{"code": m.Code})})
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
	}
	if req.Effort != "" {
		body["reasoning"] = map[string]any{"effort": req.Effort, "summary": "auto"}
	}
	if req.Tools {
		body["tools"] = []any{ToolSchema()}
	}
	resp, err := post(ctx, req.Provider, "/responses", body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return ReadSSE(resp.Body, func(e SSEEvent) error {
		return handleResponsesEvent(e, emit)
	})
}

type respEv struct {
	Type     string    `json:"type"`
	Delta    string    `json:"delta"`
	Item     *respItem `json:"item"`
	Response *struct {
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"response"`
	Message string `json:"message"`
}

type respItem struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func handleResponsesEvent(e SSEEvent, emit func(Event)) error {
	var ev respEv
	if err := json.Unmarshal([]byte(e.Data), &ev); err != nil {
		return nil
	}
	switch ev.Type {
	case "response.output_text.delta":
		emit(Event{Kind: EvText, Text: ev.Delta})
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		emit(Event{Kind: EvThinking, Text: ev.Delta})
	case "response.output_item.done":
		if ev.Item != nil && ev.Item.Type == "function_call" {
			emit(Event{Kind: EvTool, CallID: ev.Item.CallID, Code: parseCode(ev.Item.Arguments)})
		}
	case "response.completed":
		u := Usage{}
		if ev.Response != nil && ev.Response.Usage != nil {
			u = Usage{In: ev.Response.Usage.InputTokens, Out: ev.Response.Usage.OutputTokens}
		}
		emit(Event{Kind: EvDone, Usage: u})
	case "error", "response.failed":
		emit(Event{Kind: EvError, Text: ev.Message})
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
