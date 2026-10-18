package ui

import (
	"strings"

	"meno/internal/objtag"
)

// objMark brackets an object label inside text that is about to be wrapped. It
// is an invisible zero-width character, so wrapping measures the label only.
const objMark = "\u2063"

// withObjectLabels replaces well-formed object tags of a sent message by their
// labels between objMark pairs. Text without tags is returned unchanged. Old
// messages are not rewritten: this is display only.
func withObjectLabels(text string) string {
	if !strings.Contains(text, "<") {
		return text
	}
	var b strings.Builder
	for _, s := range objtag.Split(text, nil) {
		if s.Obj == nil {
			b.WriteString(strings.ReplaceAll(s.Text, objMark, ""))
			continue
		}
		b.WriteString(objMark + s.Obj.Label() + objMark)
	}
	return b.String()
}

// userLineMaker styles wrapped rows of a user message, painting the labels of
// objects in the accent colour. The state survives across rows, so a label that
// wraps stays one highlighted unit.
func userLineMaker() func(string) Line {
	inObj := false
	return func(s string) Line {
		segs := []Seg{{Text: "  ", FG: FgWhite}}
		var cur strings.Builder
		flush := func() {
			if cur.Len() == 0 {
				return
			}
			fg := FgWhite
			if inObj {
				fg = Blue
			}
			segs = append(segs, Seg{Text: cur.String(), FG: fg})
			cur.Reset()
		}
		for _, r := range s {
			if string(r) == objMark {
				flush()
				inObj = !inObj
				continue
			}
			cur.WriteRune(r)
		}
		flush()
		return Line{Segs: segs, BG: BgUser}
	}
}
