package tui

import (
	"meno/internal/render"
	"slices"
)

type savedBoundary struct {
	row      int
	separate bool
}
type liveLayout struct {
	key   turnKey
	opts  render.JSOpts
	lines []render.Line
}

type transcriptCache struct {
	key           layoutKey
	revision      uint64
	count         int
	notices       []notice
	saved         []render.Line
	boundaries    []savedBoundary
	separate      bool
	calls         map[string]bool
	lastAssistant int
	version       uint64
	frame         []render.Line
	frameVersion  uint64
	live          []liveLayout
	liveNext      int
	changed       bool
	stamp         uint64
	textStamp     uint64
	compactText   string
	text          vtext
}

func (a *App) savedTranscript(w int, js render.JSOpts) ([]render.Line, bool) {
	s, c := a.st.Session, &a.st.transcript
	key := layoutFor(w, js)
	notices := a.currentNotices()
	changed := key != c.key || c.revision != s.revision || c.count != len(s.turns) || !slices.Equal(c.notices, notices)
	if !changed {
		return c.saved, c.separate
	}
	start := 0
	if key == c.key && len(notices) == 0 && len(c.notices) == 0 && c.count <= len(s.turns) {
		start = min(s.dirtyFrom, c.count)
	}
	if start > 0 && start < len(c.boundaries) {
		b := c.boundaries[start]
		c.saved = c.saved[:b.row]
		c.separate = b.separate
		c.boundaries = c.boundaries[:start]
	} else {
		start = 0
		c.saved = c.saved[:0]
		c.boundaries = c.boundaries[:0]
		c.separate = false
		c.calls = map[string]bool{}
		c.lastAssistant = -1
	}
	appendLines := func(lines []render.Line, message bool) {
		if len(lines) == 0 {
			return
		}
		if len(c.saved) > 0 && c.separate {
			c.saved = append(c.saved, render.Band(render.BgTerm))
		}
		c.saved = append(c.saved, lines...)
		c.separate = message
	}
	for i := start; i <= len(s.turns); i++ {
		c.boundaries = append(c.boundaries, savedBoundary{len(c.saved), c.separate})
		for _, n := range notices {
			if n.After == i {
				fg := render.FgDim
				if n.Err {
					fg = render.Red
				}
				appendLines(render.WrapLines(n.Text, render.TranscriptTextWidth(w), func(text string) render.Line { return render.Plain("  "+text, fg, render.BgTerm) }), true)
			}
		}
		if i < len(s.turns) {
			t := s.turns[i]
			appendLines(a.st.lines.turnLines(t, w, js), render.TurnHasMessage(t, js))
			if t.CallID != "" {
				c.calls[t.CallID] = true
			}
			if t.Role == render.RoleAssistant {
				c.lastAssistant = i
			}
		}
	}
	// A replaced suffix can remove call IDs, so reconstruct that small index on
	// replacement; append-only histories extend the existing index above.
	if start > 0 && s.dirtyFrom < c.count {
		c.calls = map[string]bool{}
		c.lastAssistant = -1
		for i, t := range s.turns {
			if t.CallID != "" {
				c.calls[t.CallID] = true
			}
			if t.Role == render.RoleAssistant {
				c.lastAssistant = i
			}
		}
	}
	c.key, c.revision, c.count = key, s.revision, len(s.turns)
	c.notices = append(c.notices[:0], notices...)
	s.dirtyFrom = len(s.turns)
	c.version++
	return c.saved, c.separate
}

func (a *App) beginTranscript(w int, js render.JSOpts) ([]render.Line, bool) {
	saved, separate := a.savedTranscript(w, js)
	c := &a.st.transcript
	c.liveNext = 0
	c.changed = false
	if c.frameVersion != c.version {
		c.frame = append(c.frame[:0], saved...)
		c.frameVersion = c.version
		c.changed = true
	}
	return c.frame[:len(saved)], separate
}

func (a *App) liveLines(t render.Turn, w int, js render.JSOpts) []render.Line {
	c := &a.st.transcript
	i := c.liveNext
	c.liveNext++
	normalized := js
	normalized.Elapsed, normalized.Spin = 0, 0
	if i >= len(c.live) {
		c.live = append(c.live, liveLayout{})
	}
	l := &c.live[i]
	key := turnIdentity(t)
	if l.key != key || l.opts != normalized || c.changed {
		l.key, l.opts = key, normalized
		l.lines = render.BuildLines([]render.Turn{t}, w, normalized)
		c.changed = true
	}
	if t.Code != "" && js.Running && js.Exec {
		for i, line := range l.lines {
			if line.FullWidth {
				next := render.CallHeader(t, w, js)
				if line.Text() != next.Text() {
					l.lines[i] = next
					c.changed = true
				}
				break
			}
		}
	}
	return l.lines
}
func (a *App) finishTranscript(out []render.Line) []render.Line {
	c := &a.st.transcript
	if len(out) != len(c.frame) || c.changed {
		c.stamp++
	}
	c.frame = out
	if c.liveNext < len(c.live) {
		clear(c.live[c.liveNext:])
		c.live = c.live[:c.liveNext]
	}
	return out
}
