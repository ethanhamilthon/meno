package app

import (
	"strings"
	"sync"

	"meno/internal/objtag"
)

// Input objects are file, folder and skill references the user picked
// explicitly. Each one is a single private-use rune in the input buffer, so
// every editing operation — motions, operators, visual selection, registers,
// undo, wrapping, the mouse — treats it as one unit without knowing it exists.
// The rune is drawn as a short label and spelled as a tag (package objtag) when
// the message is sent or copied out.

type inputObject = objtag.Object

const (
	objBase = 0xE000
	objMax  = 0xF8FF
)

var objects = struct {
	sync.Mutex
	byRune map[rune]inputObject
	byObj  map[inputObject]rune
	next   rune
}{byRune: map[rune]inputObject{}, byObj: map[inputObject]rune{}, next: objBase}

func isObjRune(r rune) bool { return r >= objBase && r <= objMax }

// objectRune interns an object: the same object always gets the same rune. It
// fails only after thousands of distinct objects in one process.
func objectRune(o inputObject) (rune, bool) {
	objects.Lock()
	defer objects.Unlock()
	if r, ok := objects.byObj[o]; ok {
		return r, true
	}
	if objects.next > objMax {
		return 0, false
	}
	r := objects.next
	objects.next++
	objects.byRune[r], objects.byObj[o] = o, r
	return r, true
}

func objectAt(r rune) (inputObject, bool) {
	if !isObjRune(r) {
		return inputObject{}, false
	}
	objects.Lock()
	defer objects.Unlock()
	o, ok := objects.byRune[r]
	return o, ok
}

// dropUnknownObjects removes private-use runes that are not objects we made, so
// typed or pasted text cannot forge one.
func dropUnknownObjects(rs []rune) []rune {
	out := rs[:0:0]
	for _, r := range rs {
		if _, ok := objectAt(r); isObjRune(r) && !ok {
			continue
		}
		out = append(out, r)
	}
	return out
}

// externalText is rs as plain text with every object spelled as its tag: what a
// message carries and what goes to the system clipboard.
func externalText(rs []rune) string {
	var b strings.Builder
	for _, r := range rs {
		if o, ok := objectAt(r); ok {
			b.WriteString(o.Tag())
		} else if !isObjRune(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// labelText is rs as it reads on screen, objects as their labels.
func labelText(rs []rune) string {
	var b strings.Builder
	for _, r := range rs {
		if o, ok := objectAt(r); ok {
			b.WriteString(o.Label())
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// messageText is the composer content as the message text.
func (a *App) messageText() string { return externalText(a.st.Input) }

// objectKind maps a completion sigil to the kind of object it selects.
func objectKind(sigil rune) (objtag.Kind, bool) {
	switch sigil {
	case kindSkill:
		return objtag.Skill, true
	}
	return "", false
}

// objectKnown says whether a skill or mode tag may become an input object: the
// skill must be enabled and the mode must exist. Files and folders always may.
func (a *App) objectKnown(k objtag.Kind, ref string) bool {
	switch k {
	case objtag.Skill:
		return a.skillEnabled(ref)
	}
	return true
}

// runesFromText restores a message text into composer runes; only well-formed
// tags of available skills and absolute paths become objects.
func (a *App) runesFromText(text string) []rune {
	var out []rune
	for _, s := range objtag.Split(text, a.objectKnown) {
		if s.Obj == nil {
			out = append(out, dropUnknownObjects([]rune(s.Text))...)
			continue
		}
		if r, ok := objectRune(*s.Obj); ok {
			out = append(out, r)
		} else {
			out = append(out, []rune(s.Obj.Tag())...)
		}
	}
	return out
}

// labelsFromText is a message text with its tags shown as labels (the queue).
func (a *App) labelsFromText(text string) string { return labelText(a.runesFromText(text)) }
