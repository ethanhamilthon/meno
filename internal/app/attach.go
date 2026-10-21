package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"unicode"

	"meno/internal/attach"
	"meno/internal/client"
	"meno/internal/proto"
)

// kindImage marks the [Image #N] tokens in the composer.
const kindImage = 'I'

// draftImage is an image attached to the message being written. Data is nil
// once the bytes are in the attachment store.
type draftImage struct {
	attachment
	Data []byte
}

// imageReady reports an image that was read and prepared off the UI goroutine.
type imageReady struct {
	sid   string
	img   attach.Image
	err   error
	after bool // insert after the cursor (normal mode p) instead of at it
}

var imageTokenRe = regexp.MustCompile(`\[Image #(\d+)\]`)

func (a *App) attachDir() string { return filepath.Join(a.paths.Home, "attachments") }

// imageSpans are the [Image #N] tokens in the input that have an attached image.
func (a *App) imageSpans() []tokSpan { return imageSpansFor(a.st.Runtime) }

func imageSpansFor(rt *Runtime) []tokSpan {
	if len(rt.Images) == 0 {
		return nil
	}
	have := map[int]bool{}
	for _, im := range rt.Images {
		have[im.N] = true
	}
	in := rt.Input
	var out []tokSpan
	for i := 0; i < len(in); i++ {
		if in[i] != '[' {
			continue
		}
		j := i + 1
		for _, r := range "Image #" {
			if j >= len(in) || in[j] != r {
				j = -1
				break
			}
			j++
		}
		if j < 0 {
			continue
		}
		d := j
		for d < len(in) && unicode.IsDigit(in[d]) {
			d++
		}
		if d == j || d >= len(in) || in[d] != ']' {
			continue
		}
		if n, err := strconv.Atoi(string(in[j:d])); err == nil && have[n] {
			out = append(out, tokSpan{i, d + 1, kindImage, string(in[j:d])})
		}
		i = d
	}
	return out
}

// prune forgets draft images whose token is no longer in the input.
func (a *App) pruneImages() { a.pruneImagesFor(a.st.Runtime) }

func (a *App) pruneImagesFor(rt *Runtime) {
	if len(rt.Images) == 0 {
		return
	}
	live := map[int]bool{}
	for _, s := range imageSpansFor(rt) {
		n, _ := strconv.Atoi(s.name)
		live[n] = true
	}
	kept := rt.Images[:0:0]
	for _, im := range rt.Images {
		if live[im.N] {
			kept = append(kept, im)
		}
	}
	rt.Images = kept
}

// imageEndingAt is the token whose last rune is just before c.
func (a *App) imageEndingAt(c int) (tokSpan, bool) {
	for _, s := range a.imageSpans() {
		if s.end == c {
			return s, true
		}
	}
	return tokSpan{}, false
}

func (a *App) imageStartingAt(c int) (tokSpan, bool) {
	for _, s := range a.imageSpans() {
		if s.start == c {
			return s, true
		}
	}
	return tokSpan{}, false
}

// attachment is an image a message refers to.
type attachment struct {
	N    int
	Mime string
	SHA  string
	W, H int
}

// msgDraft is the text of a message with the images its tokens refer to; data
// holds the bytes of the images that are only in memory yet.
type msgDraft struct {
	Text        string
	Attachments []attachment
	data        map[string][]byte
}

// draftMessage is the text with the images its tokens refer to.
func (a *App) draftMessage(text string) msgDraft {
	m := msgDraft{Text: text, data: map[string][]byte{}}
	used := map[int]bool{}
	for _, loc := range imageTokenRe.FindAllStringSubmatch(text, -1) {
		n, _ := strconv.Atoi(loc[1])
		used[n] = true
	}
	for _, im := range a.st.Images {
		if used[im.N] {
			m.Attachments = append(m.Attachments, im.attachment)
			if im.Data != nil {
				m.data[im.SHA] = im.Data
			}
		}
	}
	sort.Slice(m.Attachments, func(i, j int) bool { return m.Attachments[i].N < m.Attachments[j].N })
	return m
}

// imagesAllowed reports whether the model of this session takes images; it
// says no only when the catalog knows that it does not.
func (a *App) imagesAllowed() bool { return a.imagesAllowedFor(a.st.Runtime) }

// recallQueue takes the queued messages back from the daemon into the composer
// for editing.
func (a *App) recallQueue() bool {
	rt := a.st.Runtime
	if len(rt.Queued) == 0 {
		return false
	}
	sid := rt.ID()
	var taken []string
	err := a.rpc(func(ctx context.Context, c *client.Client) error {
		r, e := c.Resource(ctx, "DELETE", "/v1/sessions/"+sid+"/queue", "", proto.ResourceRequest{})
		if e != nil {
			return e
		}
		var out struct {
			Queued []string `json:"queued"`
		}
		_ = json.Unmarshal(r.Data, &out)
		taken = out.Queued
		return nil
	})
	if err != nil {
		a.notifyErr("queue: " + err.Error())
		return false
	}
	rt.Queued = nil
	var back []msgDraft
	for _, t := range taken {
		back = append(back, msgDraft{Text: t})
	}
	a.restoreDraft(back)
	return len(taken) > 0
}
