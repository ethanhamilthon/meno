package mcp

import (
	"context"
	"encoding/json"
	"strings"
)

// Result is a tools/call result with its typed parts kept apart. Text is only
// the text blocks; Structured is only what the server sent as structuredContent.
type Result struct {
	Text       string
	Structured json.RawMessage // nil when the server sent none
	Content    []json.RawMessage
	IsError    bool
	Raw        json.RawMessage // the complete result as received
}

// ParseResult splits a raw tools/call result.
func ParseResult(raw json.RawMessage) (Result, error) {
	var r struct {
		Content    []json.RawMessage `json:"content"`
		Structured json.RawMessage   `json:"structuredContent"`
		IsError    bool              `json:"isError"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Result{}, err
	}
	res := Result{Content: r.Content, IsError: r.IsError, Raw: raw}
	if s := string(r.Structured); s != "" && s != "null" {
		res.Structured = r.Structured
	}
	var texts []string
	for _, c := range r.Content {
		var b struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(c, &b) == nil && b.Type == "text" {
			texts = append(texts, b.Text)
		}
	}
	res.Text = strings.Join(texts, "\n")
	return res, nil
}

// CallResult calls a tool and returns the typed result. Transport and protocol
// failures are errors; a tool that reports isError returns normally with
// IsError set.
func (c *Client) CallResult(ctx context.Context, tool string, args map[string]any) (Result, error) {
	_, ch, id, err := c.conn.call("tools/call", map[string]any{"name": tool, "arguments": args})
	if err != nil {
		return Result{}, err
	}
	raw, err := c.wait(ctx, ch, id, callTimeout)
	if err != nil {
		return Result{}, err
	}
	return ParseResult(raw)
}
