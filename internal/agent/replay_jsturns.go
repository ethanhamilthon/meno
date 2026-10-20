package agent

import (
	"encoding/json"
	"strings"

	"meno/internal/llm"
)

// ReplayJSTurns renders stored js tool turns as ordinary text messages.
// Hook point: use this when building a bash request for a session with Tool=js.
// The existing ToMsgs path and agent loop remain unchanged in this stage.
func ReplayJSTurns(turns []Turn) []llm.Msg {
	out := make([]llm.Msg, 0, len(turns)*2)
	groupText := make(map[string]string)
	for _, turn := range turns {
		if turn.ReplayGroup != "" {
			if _, ok := groupText[turn.ReplayGroup]; !ok {
				groupText[turn.ReplayGroup] = strings.TrimSpace(turn.Text)
			}
		}
		if !isStoredToolTurn(turn) {
			if turn.Role == llm.RoleUser {
				out = append(out, llm.Msg{Role: llm.RoleUser, Text: withSystem(turn)})
			} else if turn.Role == llm.RoleAssistant && turn.Text != "" {
				out = append(out, llm.Msg{Role: llm.RoleAssistant, Text: turn.Text})
			}
			continue
		}
		text := strings.TrimSpace(turn.Text)
		if turn.ReplayGroup != "" {
			text = groupText[turn.ReplayGroup]
		}
		if text != "" {
			text += "\n"
		}
		text += "ran js: " + storedCode(turn)
		out = append(out, llm.Msg{Role: llm.RoleAssistant, Text: text})
		result := "result: " + turn.Output
		if turn.Err != "" {
			result += "\nERROR: " + turn.Err
		}
		out = append(out, llm.Msg{Role: llm.RoleUser, Text: result})
	}
	return out
}

func isStoredToolTurn(turn Turn) bool {
	return turn.CallID != "" || turn.Code != "" || turn.Replay != nil && len(turn.Replay.Calls()) != 0
}

func storedCode(turn Turn) string {
	if turn.Code != "" {
		return turn.Code
	}
	if turn.Replay == nil {
		return ""
	}
	for _, call := range turn.Replay.Calls() {
		if turn.CallID != "" && call.ID != turn.CallID {
			continue
		}
		if code := decodeJSCode(call.Arguments); code != "" {
			return code
		}
		return call.Arguments
	}
	for _, raw := range turn.Replay.Responses {
		var item map[string]any
		if json.Unmarshal(raw, &item) != nil || item["type"] != "function_call" {
			continue
		}
		if id, _ := item["call_id"].(string); turn.CallID != "" && id != turn.CallID {
			continue
		}
		args, _ := item["arguments"].(string)
		if code := decodeJSCode(args); code != "" {
			return code
		}
		return args
	}
	return ""
}

func decodeJSCode(arguments string) string {
	var args map[string]any
	if json.Unmarshal([]byte(arguments), &args) != nil {
		return ""
	}
	code, _ := args["code"].(string)
	return code
}
