package web

// Service performs web searches and fetches under one policy. Search is
// optional: without a configured backend only Fetch works.
type Service struct {
	Policy  Policy
	Backend SearchBackend
}
