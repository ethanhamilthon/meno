package agent

import (
	"fmt"
	"regexp"
	"sync"
	"unicode/utf8"
)

// ElideOptions tune ElideOld: the first, cheap stage of keeping a long session
// inside the context window. Before the old part of the conversation is
// summarized by a model call, the bulky output of old tool calls is cut down to
// its start and end.
type ElideOptions struct {
	// Percent of the context window at which elision starts. 0 turns it off.
	Percent int
	// KeepRecent is how many of the newest turns are never elided (default 6).
	KeepRecent int
	// MinBytes: an output of this size or less stays whole (default 2048).
	MinBytes int
	// Head and Tail are the bytes kept from each end of an elided output (default 512).
	Head, Tail int
	// MinSavedBytes: the boundary moves only when at least this much is saved
	// (default 8192). Each move changes old messages and costs one provider
	// cache miss, so it has to pay for itself.
	MinSavedBytes int
}

func (o ElideOptions) withDefaults() ElideOptions {
	if o.KeepRecent <= 0 {
		o.KeepRecent = 6
	}
	if o.MinBytes <= 0 {
		o.MinBytes = 2048
	}
	if o.Head <= 0 {
		o.Head = 512
	}
	if o.Tail <= 0 {
		o.Tail = 512
	}
	if o.MinSavedBytes <= 0 {
		o.MinSavedBytes = 8192
	}
	return o
}

// elideState is the boundary of the elided part of a session: turns with a
// history ID below `below` are sent to the model with their large outputs cut.
// It lives in memory only. The stored turns are never changed, so the UI, the
// export and the compaction transcript still see the whole output; after a
// restart the first request rebuilds the boundary.
type elideState struct {
	below int64
	opts  ElideOptions
}

type elideHolder struct {
	mu sync.Mutex
	st elideState
}

// fullOutputMarker is the line the shell tool leaves where it dropped the middle
// of a very large output; it names the file with the whole text.
var fullOutputMarker = regexp.MustCompile(`\n\[\d+ bytes omitted; full output: [^\]\n]+\]\n`)

// ElideOld moves the boundary forward so that the large outputs of all but the
// newest turns are cut, and reports how many turns and bytes the model no
// longer receives. It does nothing when that would save less than MinSavedBytes.
func (s *Session) ElideOld(o ElideOptions) (turns, saved int) {
	o = o.withDefaults()
	all := s.Snapshot()
	cut := len(all) - o.KeepRecent
	// A response and its tool results stay on one side of the boundary.
	for cut > 0 && cut < len(all) && all[cut].ReplayGroup != "" && all[cut-1].ReplayGroup == all[cut].ReplayGroup {
		cut--
	}
	if cut <= 0 || cut >= len(all) {
		return 0, 0
	}
	boundary := all[cut].HID
	s.elide.mu.Lock()
	defer s.elide.mu.Unlock()
	if boundary <= s.elide.st.below {
		return 0, 0
	}
	for _, t := range all[:cut] {
		if t.HID < s.elide.st.below || !o.elidable(t) {
			continue
		}
		turns++
		saved += len(t.Output) - len(elideText(t.Output, o.Head, o.Tail))
	}
	if turns == 0 || saved < o.MinSavedBytes {
		return 0, 0
	}
	s.elide.st = elideState{below: boundary, opts: o}
	return turns, saved
}

// elidable: only a successful call with a large output is cut. A failure keeps
// its text, because the error is usually why the model looks at it again.
func (o ElideOptions) elidable(t Turn) bool {
	return t.HID != 0 && t.CallID != "" && t.Err == "" && len(t.Output) > o.MinBytes && len(t.Output) > 2*(o.Head+o.Tail)
}

// elidedView is the turn as the model gets it.
func (s *Session) elidedView(t Turn) Turn {
	s.elide.mu.Lock()
	st := s.elide.st
	s.elide.mu.Unlock()
	if st.below == 0 || t.HID >= st.below || !st.opts.elidable(t) {
		return t
	}
	t.Output = elideText(t.Output, st.opts.Head, st.opts.Tail)
	return t
}

// elideText keeps the first head and the last tail bytes of out, cut at
// character boundaries, and says how much is missing. The marker of the shell
// tool, which names the file with the whole output, is kept.
func elideText(out string, head, tail int) string {
	h := head
	for h > 0 && h < len(out) && !utf8.RuneStart(out[h]) {
		h--
	}
	t := len(out) - tail
	for t < len(out) && t > h && !utf8.RuneStart(out[t]) {
		t++
	}
	if t <= h {
		return out
	}
	omitted := t - h
	if m := fullOutputMarker.FindString(out[h:t]); m != "" {
		return out[:h] + fmt.Sprintf("\n[... %d bytes omitted from the context to save space ...]", omitted) + m + out[t:]
	}
	return out[:h] + fmt.Sprintf("\n[... %d bytes omitted from the context to save space; run the command again if you need them ...]\n", omitted) + out[t:]
}
