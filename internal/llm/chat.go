package llm

import (
	"context"
	"encoding/json"
)

type chatClient struct{}

func chatMessages(system string, msgs []Msg) []map[string]any {
	out := []map[string]any{{"role": "system", "content": system}}
	for _, m := range msgs {
		switch {
		case m.Role == RoleTool:
			out = append(out, map[string]any{"role": "tool", "tool_call_id": m.CallID, "content": m.Text})
		case m.Role == RoleAssistant && m.Code != "":
			out = append(out, map[string]any{
				"role":    "assistant",
				"content": m.Text,
				"tool_calls": []any{map[string]any{
					"id": m.CallID, "type": "function",
					"function": map[string]any{"name": ToolName, "arguments": mustJSON(map[string]string{"code": m.Code})},
				}},
			})
		default:
			out = append(out, map[string]any{"role": m.Role, "content": m.Text})
		}
	}
	return out
}

func (chatClient) Stream(ctx context.Context, req Request, emit func(Event)) error {
	body := map[string]any{
		"model":          req.Model,
		"messages":       chatMessages(req.System, req.Msgs),
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}
	if req.Effort != "" {
		body["reasoning_effort"] = req.Effort
	}
	if req.Tools {
		body["tools"] = []any{map[string]any{"type": "function", "function": ToolSchema()}}
	}
	resp, err := post(ctx, req.Provider, "/chat/completions", body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	calls := map[int]*chatCall{}
	var usage Usage

	err = ReadSSE(resp.Body, func(e SSEEvent) error {
		if e.Data == "[DONE]" {
			return nil
		}
		var c chatChunk
		if json.Unmarshal([]byte(e.Data), &c) != nil {
			return nil
		}
		if c.Error != nil {
			emit(Event{Kind: EvError, Text: c.Error.Message})
			return nil
		}
		for _, ch := range c.Choices {
			if ch.Delta.Content != "" {
				emit(Event{Kind: EvText, Text: ch.Delta.Content})
			}
			if ch.Delta.Reasoning != "" {
				emit(Event{Kind: EvThinking, Text: ch.Delta.Reasoning})
			}
			for _, tc := range ch.Delta.ToolCalls {
				cc := calls[tc.Index]
				if cc == nil {
					cc = &chatCall{}
					calls[tc.Index] = cc
				}
				if tc.ID != "" {
					cc.id = tc.ID
				}
				cc.args += tc.Function.Arguments
			}
		}
		if c.Usage != nil {
			usage = Usage{In: c.Usage.PromptTokens, Out: c.Usage.CompletionTokens}

		}
		return nil
	})
	for i := 0; i < len(calls); i++ {
		if cc := calls[i]; cc != nil {
			emit(Event{Kind: EvTool, CallID: cc.id, Code: parseCode(cc.args)})
		}
	}
	emit(Event{Kind: EvDone, Usage: usage})
	return err
}

type chatCall struct {
	id   string
	args string
}

type chatChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning_content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}
