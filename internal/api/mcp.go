package api

import (
	"encoding/json"
)

// MCPResult preserves tool data independently of transport failures.
type MCPResult struct {
	Text              string          `json:"text,omitempty"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	ResultID          string          `json:"resultId,omitempty"`
	IsError           bool            `json:"isError"`
	Artifacts         []MCPArtifact   `json:"artifacts,omitempty"`
	Size              int             `json:"size,omitempty"`
	Notes             []string        `json:"notes,omitempty"`
}

type MCPArtifact struct {
	Index int    `json:"index"`
	Kind  string `json:"kind"`
	Mime  string `json:"mime"`
	Size  int    `json:"size"`
	Path  string `json:"path"`
	URI   string `json:"uri,omitempty"`
}
