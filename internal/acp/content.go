package acp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"meno/internal/api"
	"meno/internal/content"
)

// maxImageBytes bounds one decoded image block.
const maxImageBytes = 20 << 20

// maxEmbeddedText bounds one embedded text resource.
const maxEmbeddedText = 256 << 10

// block is one ACP content block. Only the fields Meno reads are decoded.
type block struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	URI      string `json:"uri,omitempty"`
	Name     string `json:"name,omitempty"`
	Resource *struct {
		URI      string  `json:"uri"`
		MimeType string  `json:"mimeType,omitempty"`
		Text     *string `json:"text,omitempty"`
		Blob     *string `json:"blob,omitempty"`
	} `json:"resource,omitempty"`
}

// promptInput is the text and attachments of a prompt.
type promptInput struct {
	Text        string
	Attachments []api.Attachment
}

// parsePrompt converts content blocks into a message. Content Meno cannot
// represent is rejected with a clear error, never dropped.
func parsePrompt(blocks []block) (promptInput, error) {
	var in promptInput
	var parts []string
	for i, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "image":
			raw, err := base64.StdEncoding.DecodeString(b.Data)
			if err != nil {
				return in, errParams("prompt[%d]: image data is not valid base64", i)
			}
			if len(raw) == 0 || len(raw) > maxImageBytes {
				return in, errParams("prompt[%d]: image must be between 1 byte and %d MiB", i, maxImageBytes>>20)
			}
			if !strings.HasPrefix(b.MimeType, "image/") {
				return in, errParams("prompt[%d]: unsupported image type %q", i, b.MimeType)
			}
			name := fmt.Sprintf("image-%d%s", len(in.Attachments)+1, content.Ext(b.MimeType))
			in.Attachments = append(in.Attachments, api.Attachment{Name: name, MIME: b.MimeType, Data: b.Data})
		case "resource":
			if b.Resource == nil || b.Resource.Text == nil {
				return in, errParams("prompt[%d]: only text resources can be embedded", i)
			}
			if len(*b.Resource.Text) > maxEmbeddedText {
				return in, errParams("prompt[%d]: embedded resource is larger than %d KiB", i, maxEmbeddedText>>10)
			}
			parts = append(parts, fmt.Sprintf("Context from %s:\n```\n%s\n```", b.Resource.URI, *b.Resource.Text))
		case "resource_link":
			// A link is only a reference; nothing is fetched or read.
			parts = append(parts, fmt.Sprintf("Referenced resource: %s (%s)", b.URI, b.Name))
		default:
			return in, errParams("prompt[%d]: unsupported content type %q", i, b.Type)
		}
	}
	in.Text = strings.Join(parts, "\n\n")
	if strings.TrimSpace(in.Text) == "" && len(in.Attachments) == 0 {
		return in, errParams("prompt is empty")
	}
	return in, nil
}

func textContent(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func chunk(kind, text string) map[string]any {
	return map[string]any{"sessionUpdate": kind, "content": textContent(text)}
}

// toolStart maps a started Bash call to an ACP tool call.
func toolStart(id, command, title string) map[string]any {
	if title == "" {
		title = firstLine(command)
	}
	return map[string]any{
		"sessionUpdate": "tool_call", "toolCallId": id, "title": title,
		"kind": "execute", "status": "in_progress",
		"rawInput": map[string]any{"command": command},
	}
}

// maxToolOutput bounds the output sent in one tool update.
const maxToolOutput = 64 << 10

// toolEnd maps a finished call to a tool_call_update.
func toolEnd(id, output string, exit int, taskID string) map[string]any {
	status := "completed"
	if exit != 0 {
		status = "failed"
	}
	if len(output) > maxToolOutput {
		output = output[:maxToolOutput] + "\n[output truncated]"
	}
	raw := map[string]any{"exit_code": exit}
	if taskID != "" {
		raw["task_id"] = taskID
	}
	u := map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": id, "status": status, "rawOutput": raw}
	if output != "" {
		u["content"] = []any{map[string]any{"type": "content", "content": textContent(output)}}
	}
	return u
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

// historyUpdates maps persisted turns to the updates that replay them.
func historyUpdates(turns []api.Turn) []map[string]any {
	var out []map[string]any
	for _, t := range turns {
		switch t.Role {
		case "user":
			if t.Content != "" && t.Kind == "" {
				out = append(out, chunk("user_message_chunk", t.Content))
			}
		case "assistant":
			out = append(out, assistantUpdates(t, "", "", nil)...)
		}
	}
	return out
}

// assistantUpdates maps one assistant turn, skipping what was already sent as
// live text, thinking or calls.
func assistantUpdates(t api.Turn, sentText, sentThink string, seen map[string]bool) []map[string]any {
	var out []map[string]any
	if rest, ok := remainder(t.Thinking, sentThink); ok && rest != "" {
		out = append(out, chunk("agent_thought_chunk", rest))
	}
	text := t.Content
	var calls []api.CallState
	if len(t.Parts) > 0 {
		var sb strings.Builder
		for _, p := range t.Parts {
			switch p.Type {
			case "text":
				sb.WriteString(p.Text)
			case "call":
				if p.Call != nil {
					calls = append(calls, *p.Call)
				}
			}
		}
		if sb.Len() > 0 {
			text = sb.String()
		}
	}
	if rest, ok := remainder(text, sentText); ok && rest != "" {
		out = append(out, chunk("agent_message_chunk", rest))
	}
	for _, c := range calls {
		if seen != nil && seen[c.CallID] {
			continue
		}
		out = append(out, toolStart(c.CallID, c.Command, c.Title))
		if c.Exit != nil {
			out = append(out, toolEnd(c.CallID, c.Output, *c.Exit, c.TaskID))
		}
	}
	return out
}

// remainder returns the part of full that was not yet sent. When sent is not a
// prefix of full the whole text is returned: the live text was replaced.
func remainder(full, sent string) (string, bool) {
	if strings.HasPrefix(full, sent) {
		return full[len(sent):], true
	}
	return full, true
}

func mustJSON(v any) json.RawMessage {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return bytes.TrimRight(b.Bytes(), "\n")
}
