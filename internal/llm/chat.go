package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type chatClient struct{}

func chatMessages(system string, msgs []Msg) []map[string]any {
	out := []map[string]any{{"role": "system", "content": system}}
	for _, m := range msgs {
		switch {
		case m.Role == RoleTool:
			out = append(out, map[string]any{"role": "tool", "tool_call_id": m.CallID, "content": m.Text})
		case m.Role == RoleAssistant && len(m.Calls()) != 0:
			var calls []any
			for _, c := range m.Calls() {
				calls = append(calls, map[string]any{"id": c.ID, "type": "function", "function": map[string]any{"name": c.Name, "arguments": c.Arguments}})
			}
			out = append(out, map[string]any{"role": "assistant", "content": m.Text, "tool_calls": calls})
		case m.Role == RoleUser:
			out = append(out, map[string]any{"role": m.Role, "content": userContent(m, "text", "image_url", true)})
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
		body["tools"] = []any{map[string]any{"type": "function", "function": ToolSchema(req.ToolDescription, req.ModelTimeout)}}
	}
	applyCache(body, req)
	resp, err := post(ctx, req.Provider, "/chat/completions", body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	calls := map[int]*chatCall{}
	var usage Usage
	finished, done := false, false

	err = ReadSSE(resp.Body, func(e SSEEvent) error {
		if e.Data == "[DONE]" {
			done = true
			return nil
		}
		if done {
			return fmt.Errorf("chat event after DONE")
		}
		var c chatChunk
		if err := json.Unmarshal([]byte(e.Data), &c); err != nil {
			return fmt.Errorf("chat stream: %w", err)
		}
		if c.Error != nil {
			emit(Event{Kind: EvError, Text: c.Error.Message})
			return fmt.Errorf("chat stream: %s", c.Error.Message)
		}
		for _, ch := range c.Choices {
			if finished {
				return fmt.Errorf("chat choice after finish reason")
			}
			if ch.FinishReason != "" {
				switch ch.FinishReason {
				case "stop", "tool_calls", "function_call":
					finished = true
				default:
					return fmt.Errorf("chat stream finish reason: %s", ch.FinishReason)
				}
			}
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
				if tc.Function.Name != "" {
					cc.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					emit(Event{Kind: EvToolDelta, CallID: cc.id, Code: PartialCode(cc.args)})
				}
			}
		}
		if c.Usage != nil {
			usage = Usage{In: c.Usage.PromptTokens, Out: c.Usage.CompletionTokens}
			if d := c.Usage.Details; d != nil && (d.Cached != nil || d.CacheWrite != nil) {
				usage.CacheReported = true
				if d.Cached != nil {
					usage.Cached = *d.Cached
				}
				if d.CacheWrite != nil {
					usage.CacheWrite = *d.CacheWrite
				}
			}

		}
		return nil
	})
	if err != nil {
		return err
	}
	if !finished && !done {
		return fmt.Errorf("chat stream: %w", io.ErrUnexpectedEOF)
	}
	for _, cc := range calls {
		if cc.id == "" {
			return fmt.Errorf("chat function call missing id")
		}
	}
	indices := make([]int, 0, len(calls))
	for i := range calls {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	for _, i := range indices {
		if cc := calls[i]; cc != nil {
			emit(Event{Kind: EvTool, CallID: cc.id, Name: cc.name, Arguments: cc.args, Code: parseCode(cc.args)})
		}
	}
	emit(Event{Kind: EvDone, Usage: usage})
	return nil
}

type chatCall struct {
	name string
	id   string
	args string
}

type chatChunk struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Delta        struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning_content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Arguments string `json:"arguments"`
					Name      string `json:"name"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		Details *struct {
			Cached     *int `json:"cached_tokens"`
			CacheWrite *int `json:"cache_write_tokens"`
		} `json:"prompt_tokens_details"`
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}
