// Package objtag is the text form of input objects: file and folder
// references and explicit skill selections. A message carries them as tags;
// the composer and the timeline show them as short labels.
package objtag

import (
	"html"
	"path/filepath"
	"strings"
)

// Kind of an object.
type Kind string

const (
	File   Kind = "file"
	Folder Kind = "folder"
	Skill  Kind = "skill"
	Mode   Kind = "mode"
)

// Check says whether a skill or mode reference may stay an object: it is the
// caller's list of what is known (nil accepts any name).
type Check func(k Kind, ref string) bool

// Object is a fixed reference. For files and folders Ref is an absolute path,
// exactly as picked (symlinks are not resolved); for a skill it is its name.
type Object struct {
	Kind Kind
	Ref  string
}

func esc(s string) string { return html.EscapeString(s) }

// Tag is the canonical text of the object; &, < and > are escaped so the
// content can never close or forge a tag.
func (o Object) Tag() string {
	switch o.Kind {
	case Skill:
		return "<skill>" + esc(o.Ref) + "</skill>"
	case Mode:
		return "<mode>" + esc(o.Ref) + "</mode>"
	case Folder:
		return "<folder-link>" + esc(o.Ref) + "</folder-link>"
	}
	return "<file-link>" + esc(o.Ref) + "</file-link>"
}

// Label is the short form shown on screen.
func (o Object) Label() string {
	switch o.Kind {
	case Skill:
		return "/" + o.Ref
	case Mode:
		return "#" + o.Ref
	case Folder:
		return "@" + filepath.Base(o.Ref) + "/"
	}
	return "@" + filepath.Base(o.Ref)
}

// Segment is a run of plain text or one object.
type Segment struct {
	Text string
	Obj  *Object
}

var kinds = []struct {
	open, close string
	kind        Kind
}{
	{"<file-link>", "</file-link>", File},
	{"<folder-link>", "</folder-link>", Folder},
	{"<skill>", "</skill>", Skill},
	{"<mode>", "</mode>", Mode},
}

// Split cuts text into plain runs and objects. Only well-formed, complete tags
// qualify: file and folder paths must be absolute and single-line, and a skill
// or mode must pass ok (nil accepts any name). Tags inside code spans or fences,
// and anything relative, malformed or unclosed, stay plain text. Untagged paths
// and /names are always plain.
func Split(text string, ok Check) []Segment {
	var segs []Segment
	start := 0
	flush := func(end int) {
		if end > start {
			segs = append(segs, Segment{Text: text[start:end]})
		}
	}
	inFence, inSpan := false, false
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "```"):
			inFence = !inFence
			i += 3
			continue
		case text[i] == '`' && !inFence:
			inSpan = !inSpan
		case text[i] == '\n':
			inSpan = false
		}
		if text[i] == '<' && !inFence && !inSpan {
			if o, n, found := tagAt(text[i:], ok); found {
				flush(i)
				segs = append(segs, Segment{Obj: &o})
				i += n
				start = i
				continue
			}
		}
		i++
	}
	flush(len(text))
	return segs
}

func tagAt(s string, ok Check) (Object, int, bool) {
	for _, k := range kinds {
		if !strings.HasPrefix(s, k.open) {
			continue
		}
		end := strings.Index(s, k.close)
		if end < 0 {
			return Object{}, 0, false
		}
		ref := html.UnescapeString(s[len(k.open):end])
		if ref == "" || strings.ContainsAny(ref, "\n\r") {
			return Object{}, 0, false
		}
		if k.kind == File || k.kind == Folder {
			if !filepath.IsAbs(ref) {
				return Object{}, 0, false
			}
		} else if ok != nil && !ok(k.kind, ref) {
			return Object{}, 0, false
		}
		return Object{k.kind, ref}, end + len(k.close), true
	}
	return Object{}, 0, false
}
