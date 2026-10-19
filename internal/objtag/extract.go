package objtag

import "strings"

// Extract removes the objects for which take returns true and returns them in
// order of appearance together with the remaining text. Every other object is
// kept in the remaining text as its canonical Tag(); plain runs are kept as is.
// The remaining text is not trimmed. It is built on Split, so the parsing rules
// are the same (code spans, fences, absolute paths, the ok check). A nil take
// takes nothing.
func Extract(text string, ok Check, take func(Object) bool) (taken []Object, rest string) {
	var kept strings.Builder
	for _, s := range Split(text, ok) {
		switch {
		case s.Obj == nil:
			kept.WriteString(s.Text)
		case take != nil && take(*s.Obj):
			taken = append(taken, *s.Obj)
		default:
			kept.WriteString(s.Obj.Tag())
		}
	}
	return taken, kept.String()
}
