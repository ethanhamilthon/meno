package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"meno/internal/attach"
	"meno/internal/client"
	"meno/internal/proto"
	"meno/internal/ui"
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
func (a *App) imageSpans() []tokSpan {
	if len(a.st.Images) == 0 {
		return nil
	}
	have := map[int]bool{}
	for _, im := range a.st.Images {
		have[im.N] = true
	}
	in := a.st.Input
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
func (a *App) pruneImages() {
	if len(a.st.Images) == 0 {
		return
	}
	live := map[int]bool{}
	for _, s := range a.imageSpans() {
		n, _ := strconv.Atoi(s.name)
		live[n] = true
	}
	kept := a.st.Images[:0:0]
	for _, im := range a.st.Images {
		if live[im.N] {
			kept = append(kept, im)
		}
	}
	a.st.Images = kept
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
func (a *App) imagesAllowed() bool {
	if info, ok := a.modelInfo(); ok && info.Vision != nil && !*info.Vision {
		a.notifyErr("model " + modelOf(a) + " does not accept images")
		return false
	}
	return true
}

// setDraft replaces the composer text and its images.
func (a *App) setDraft(m msgDraft, data map[string][]byte) {
	a.setInput(a.runesFromText(m.Text))
	a.st.Images = nil
	for _, at := range m.Attachments {
		a.st.Images = append(a.st.Images, draftImage{attachment: at, Data: data[at.SHA]})
	}
}

// restoreDraft puts queued messages back into the composer, after what is
// already there.
func (a *App) restoreDraft(rest []msgDraft) {
	if len(rest) == 0 {
		return
	}
	a.pruneImages()
	data := map[string][]byte{}
	for _, im := range a.st.Images {
		data[im.SHA] = im.Data
	}
	for _, m := range rest {
		for k, v := range m.data {
			data[k] = v
		}
	}
	var texts []string
	var atts []attachment
	if cur := a.messageText(); strings.TrimSpace(cur) != "" {
		texts = append(texts, cur)
	}
	for _, m := range rest {
		texts = append(texts, m.Text)
		atts = append(atts, m.Attachments...)
	}
	a.setDraft(msgDraft{Text: strings.Join(texts, "\n\n"), Attachments: atts}, data)
}

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

// insertImage adds a prepared image to the draft and puts its token in the
// input.
func (a *App) insertImage(img attach.Image, after bool) {
	if a.st.Ask != nil {
		a.notifyErr("can't attach an image while the agent asks a question")
		return
	}
	a.pruneImages()
	if len(a.st.Images) >= attach.MaxImages {
		a.notifyErr("at most " + itoa(attach.MaxImages) + " images per message")
		return
	}
	if !a.imagesAllowed() {
		return
	}
	n := 1
	for _, im := range a.st.Images {
		n = max(n, im.N+1)
	}
	a.st.Images = append(a.st.Images, draftImage{
		attachment: attachment{N: n, Mime: img.Mime, SHA: img.SHA, W: img.W, H: img.H},
		Data:       img.Data,
	})
	normal := a.st.Mode == ui.ModeNormal
	tok := []rune("[Image #" + itoa(n) + "]")
	a.clampCursor()
	if normal {
		a.pushUndo(a.snapshot())
		if after && a.st.Cursor < len(a.st.Input) {
			a.moveRight()
		}
		a.insertAtCursor(tok)
		a.st.Cursor--
		return
	}
	a.insertAtCursor(append(tok, ' '))
}
