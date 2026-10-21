package proto

// WebFetchRequest asks the daemon to fetch one page.
type WebFetchRequest struct {
	URL string `json:"url"`
}

// WebSearchRequest asks the daemon to search with the configured provider.
type WebSearchRequest struct {
	Query string `json:"query"`
}
