package app

import (
	"strings"

	"meno/internal/modes"
	"meno/internal/objtag"
)

// A message can name modes (#subagents, #goal, ...), chosen from the menu like
// a skill. The names of a message replace the modes of its session; a message
// without modes keeps them, and #none clears them. The daemon applies them.

// messageModes reads the modes a message names: the <mode> objects of its text,
// in order, and the rest of the text. "none" clears the set: it is returned as
// no name, so the caller empties the set. tagged is false when the message
// names no mode at all, and then the set stays as it is.
func (a *App) messageModes(text string) (names []string, rest string, tagged bool) {
	known := func(k objtag.Kind, ref string) bool {
		if k != objtag.Mode {
			return false
		}
		return ref == modes.Control || a.objectKnown(k, ref)
	}
	taken, kept := objtag.Extract(text, known, func(o objtag.Object) bool { return o.Kind == objtag.Mode })
	if len(taken) == 0 {
		return nil, text, false
	}
	for _, o := range taken {
		if o.Ref != modes.Control && !hasName(names, o.Ref) {
			names = append(names, o.Ref)
		}
	}
	return names, strings.TrimSpace(kept), true
}

// modesNotice is the line that says which modes a session runs in now.
func modesNotice(names []string) string {
	if len(names) == 0 {
		return "modes off"
	}
	s := "modes:"
	for _, n := range names {
		s += " #" + n
	}
	return s
}
