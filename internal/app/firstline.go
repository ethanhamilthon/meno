package app

import "strings"

// firstLine is the start of a message cut to n runes, the way the daemon names
// a session after its first message.
func firstLine(text string, n int) string {
	text = strings.TrimSpace(text)
	if r := []rune(text); len(r) > n {
		return string(r[:n])
	}
	return text
}
