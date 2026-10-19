package prompt

import "strings"

// element wraps body in an XML-like element called name. Trailing newlines of
// the body are trimmed. Every section of the system prompt is one element, so
// the model can tell where meno's text ends and the user's AGENTS.md begins.
func element(name, body string) string {
	return "<" + name + ">\n" + strings.TrimRight(escapeBody(body), "\n") + "\n</" + name + ">"
}

// escapeBody keeps text from closing an element early: every "</" becomes
// "&lt;/". Nothing else is escaped, so the user's files stay readable.
func escapeBody(s string) string {
	return strings.ReplaceAll(s, "</", "&lt;/")
}
