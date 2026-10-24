package engine

// lastAuthorCounts is default-deny: only the last accepted TUI message counts.
// Callers hold the session commit and state locks when reading run authors.
func lastAuthorCounts(authors []bool) bool {
	return len(authors) > 0 && authors[len(authors)-1]
}
