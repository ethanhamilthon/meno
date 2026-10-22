package api

// ExportResponse is the human-only session export resource. The daemon renders
// credential-free Markdown after releasing session locks; clients only write it.
type ExportResponse struct {
	Markdown string `json:"markdown"`
}
