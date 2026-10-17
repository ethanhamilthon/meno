package app

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/attach"
	"meno/internal/ui"
)

// kindImage marks the [Image #N] tokens in the composer.
const kindImage = 'I'

// draftImage is an image attached to the message being written. Data is nil
// once the bytes are in the attachment store.
type draftImage struct {
	agent.Attachment
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

// draftMessage is the text with the images its tokens refer to.
func (a *App) draftMessage(text string) agent.Message {
	m := agent.Message{Text: text}
	used := map[int]bool{}
	for _, loc := range imageTokenRe.FindAllStringSubmatch(text, -1) {
		n, _ := strconv.Atoi(loc[1])
		used[n] = true
	}
	for _, im := range a.st.Images {
		if used[im.N] {
			m.Attachments = append(m.Attachments, im.Attachment)
		}
	}
	sort.Slice(m.Attachments, func(i, j int) bool { return m.Attachments[i].N < m.Attachments[j].N })
	return m
}

// storeImages writes the bytes of the attachments of m that are only in memory.
func (a *App) storeImages(m agent.Message) error {
	st := attach.Store{Dir: a.attachDir()}
	for _, im := range a.st.Images {
		for _, at := range m.Attachments {
			if at.N == im.N && im.Data != nil {
				err := st.Put(attach.Image{Data: im.Data, Mime: im.Mime, W: im.W, H: im.H, SHA: im.SHA})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
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
func (a *App) setDraft(m agent.Message, data map[string][]byte) {
	a.setInput([]rune(m.Text))
	a.st.Images = nil
	for _, at := range m.Attachments {
		a.st.Images = append(a.st.Images, draftImage{Attachment: at, Data: data[at.SHA]})
	}
}

// restoreDraft puts queued messages back into the composer, after what is
// already there.
func (a *App) restoreDraft(all []agent.Message) {
	var rest []agent.Message
	for _, m := range all {
		if !m.Viewed { // images the agent asked for are not the user's to edit
			rest = append(rest, m)
		}
	}
	if len(rest) == 0 {
		return
	}
	a.pruneImages()
	data := map[string][]byte{}
	for _, im := range a.st.Images {
		data[im.SHA] = im.Data
	}
	var msgs []agent.Message
	if cur := string(a.st.Input); strings.TrimSpace(cur) != "" {
		msgs = append(msgs, a.draftMessage(cur))
	}
	a.setDraft(agent.Combine(append(msgs, rest...)), data)
}

// recallQueue moves the queued messages back into the composer for editing.
func (a *App) recallQueue() bool {
	rest := a.st.Queue.TakeWritten()
	a.restoreDraft(rest)
	return len(rest) > 0
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
		Attachment: agent.Attachment{N: n, Mime: img.Mime, SHA: img.SHA, W: img.W, H: img.H},
		Data:       img.Data,
	})
	normal := a.st.Mode == ui.ModeNormal
	tok := []rune(agent.ImageToken(n))
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

// attachAsync prepares an image in the background and inserts it when ready.
func (a *App) attachAsync(after bool, load func() (attach.Image, error)) {
	sid := a.st.ID()
	go func() {
		img, err := load()
		a.postEvent(imageReady{sid: sid, img: img, err: err, after: after})
	}()
}

func (a *App) attachFromClipboard(after bool) {
	read := a.clipboard
	if read == nil {
		read = func() ([]byte, error) { return attach.Clipboard(runtime.GOOS) }
	}
	a.attachAsync(after, func() (attach.Image, error) {
		data, err := read()
		if err != nil {
			return attach.Image{}, err
		}
		return attach.Prepare(data)
	})
}

func (a *App) attachFile(path string, after bool) {
	a.attachAsync(after, func() (attach.Image, error) {
		st, err := os.Stat(path)
		if err != nil {
			return attach.Image{}, err
		}
		if !st.Mode().IsRegular() {
			return attach.Image{}, errors.New(path + " is not a file")
		}
		if st.Size() > attach.MaxBytes {
			return attach.Image{}, errors.New("image is larger than " + itoa(attach.MaxBytes>>20) + " MB")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return attach.Image{}, err
		}
		return attach.Prepare(data)
	})
}

func (a *App) onImageReady(m imageReady) {
	a.inRT(m.sid, func() {
		if m.err != nil {
			a.notifyErr(m.err.Error())
			return
		}
		a.insertImage(m.img, m.after)
	})
}

// attachCommand implements :attach [path]: a file, or the clipboard without an argument.
func (a *App) attachCommand(arg string) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		a.attachFromClipboard(false)
		return
	}
	if p, ok := attach.PathFromText(arg, userHome()); ok {
		arg = p
	} else if strings.HasPrefix(arg, "~/") {
		arg = filepath.Join(userHome(), arg[2:])
	}
	if !filepath.IsAbs(arg) {
		arg = filepath.Join(a.st.Cwd, arg)
	}
	a.attachFile(arg, false)
}

func userHome() string {
	h, _ := os.UserHomeDir()
	return h
}

// onPaste handles the start and end of a bracketed paste. The text between
// them arrives as key events and is collected, not typed.
func (a *App) onPaste(e *tcell.EventPaste) {
	if e.Start() {
		a.pasting = a.st.Over == OverNone && a.st.Mode == ui.ModeInsert && !a.askChoosing()
		a.pasteBuf = a.pasteBuf[:0]
		return
	}
	if !a.pasting {
		return
	}
	a.pasting = false
	text := strings.ReplaceAll(strings.ReplaceAll(string(a.pasteBuf), "\r\n", "\n"), "\r", "\n")
	a.pasteBuf = a.pasteBuf[:0]
	switch {
	case text == "":
		// Terminals send an empty paste when the clipboard holds no text.
		a.attachFromClipboard(false)
	default:
		if p, ok := attach.PathFromText(text, userHome()); ok && a.st.Ask == nil {
			a.attachFile(p, false)
			return
		}
		a.clampCursor()
		a.insertAtCursor([]rune(text))
	}
}

func (a *App) collectPaste(k *tcell.EventKey) {
	switch k.Key() {
	case tcell.KeyRune:
		a.pasteBuf = append(a.pasteBuf, k.Rune())
	case tcell.KeyEnter, tcell.KeyCtrlJ:
		a.pasteBuf = append(a.pasteBuf, '\n')
	case tcell.KeyTab:
		a.pasteBuf = append(a.pasteBuf, '\t')
	}
}
