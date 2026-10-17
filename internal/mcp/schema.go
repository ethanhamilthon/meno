package mcp

import (
	"encoding/json"
	"sort"
	"strings"
)

// Signature summarizes the input schema of a tool as a JS object type, for
// example "{query: string, limit?: number}". Unknown shapes give "{...}".
func (t Tool) Signature() string {
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if len(t.InputSchema) == 0 || json.Unmarshal(t.InputSchema, &s) != nil {
		return "{...}"
	}
	if len(s.Properties) == 0 {
		return ""
	}
	req := map[string]bool{}
	for _, r := range s.Required {
		req[r] = true
	}
	names := make([]string, 0, len(s.Properties))
	for n := range s.Properties {
		names = append(names, n)
	}
	// Required arguments first, then alphabetical.
	sort.Slice(names, func(i, j int) bool {
		if req[names[i]] != req[names[j]] {
			return req[names[i]]
		}
		return names[i] < names[j]
	})
	parts := make([]string, len(names))
	for i, n := range names {
		opt := "?"
		if req[n] {
			opt = ""
		}
		parts[i] = n + opt + ": " + schemaType(s.Properties[n])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func schemaType(raw json.RawMessage) string {
	var p struct {
		Type  json.RawMessage   `json:"type"`
		Enum  []json.RawMessage `json:"enum"`
		Items json.RawMessage   `json:"items"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return "any"
	}
	if n := len(p.Enum); n > 0 && n <= 6 {
		vals := make([]string, n)
		for i, e := range p.Enum {
			vals[i] = string(e)
		}
		return strings.Join(vals, " | ")
	}
	var one string
	if json.Unmarshal(p.Type, &one) == nil {
		switch one {
		case "integer":
			return "number"
		case "array":
			if len(p.Items) > 0 {
				return schemaType(p.Items) + "[]"
			}
			return "any[]"
		case "":
			return "any"
		}
		return one
	}
	var many []string
	if json.Unmarshal(p.Type, &many) == nil && len(many) > 0 {
		return strings.Join(many, " | ")
	}
	return "any"
}
