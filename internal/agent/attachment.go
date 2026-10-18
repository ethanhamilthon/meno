package agent

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"meno/internal/attach"
	"meno/internal/llm"
)

// Attachment is an image sent with a user message. The message text refers to
// it by the token [Image #N]; the bytes live in the attachment store under
// SHA.
type Attachment struct {
	N    int    `json:"n"`
	Mime string `json:"mime"`
	SHA  string `json:"sha"`
	W    int    `json:"w"`
	H    int    `json:"h"`
}

// Message is a user message in the making: text plus the images it refers to.
type Message struct {
	Text        string
	Attachments []Attachment
	// Viewed marks an image the agent asked to see with view(); it is not
	// something the user wrote.
	Viewed bool
}

var imageTokenRe = regexp.MustCompile(`\[Image #(\d+)\]`)

// ImageToken is the text that stands for image n in a message.
func ImageToken(n int) string { return "[Image #" + strconv.Itoa(n) + "]" }

// Combine joins messages into one (separated by a blank line) and numbers the
// images 1, 2, 3... across all of them.
func Combine(ms []Message) Message {
	var out Message
	var texts []string
	for _, m := range ms {
		renum := map[int]int{}
		for _, at := range m.Attachments {
			at2 := at
			at2.N = len(out.Attachments) + 1
			renum[at.N] = at2.N
			out.Attachments = append(out.Attachments, at2)
		}
		text := imageTokenRe.ReplaceAllStringFunc(m.Text, func(tok string) string {
			n, _ := strconv.Atoi(imageTokenRe.FindStringSubmatch(tok)[1])
			if to, ok := renum[n]; ok {
				return ImageToken(to)
			}
			return tok
		})
		texts = append(texts, text)
	}
	out.Text = strings.Join(texts, "\n\n")
	return out
}

// AttachmentPath is where the bytes of an attachment are stored.
func (s *Session) AttachmentPath(a Attachment) string {
	return filepath.Join(s.AttachDir, a.SHA+attach.Ext(a.Mime))
}

// TextWithImagePaths is the text of t with every image token replaced by what
// show makes of the path of the stored image.
func (s *Session) TextWithImagePaths(t Turn, show func(path string) string) string {
	if len(t.Attachments) == 0 {
		return t.Text
	}
	byN := map[int]Attachment{}
	for _, a := range t.Attachments {
		byN[a.N] = a
	}
	return imageTokenRe.ReplaceAllStringFunc(t.Text, func(tok string) string {
		n, _ := strconv.Atoi(imageTokenRe.FindStringSubmatch(tok)[1])
		if a, ok := byN[n]; ok {
			return show(s.AttachmentPath(a))
		}
		return tok
	})
}

// omitOldImages keeps the newest limit images in the request; older ones are
// replaced by a note in the text. It must stay a no-op below the limit.
func omitOldImages(msgs []llm.Msg, limit int) {
	left := limit
	for i := len(msgs) - 1; i >= 0; i-- {
		m := &msgs[i]
		if len(m.Images) == 0 {
			continue
		}
		if left >= len(m.Images) {
			left -= len(m.Images)
			continue
		}
		// Keep the later images of this message, drop the earlier ones.
		drop := len(m.Images) - left
		gone := map[int]bool{}
		for _, im := range m.Images[:drop] {
			gone[im.N] = true
		}
		m.Text = imageTokenRe.ReplaceAllStringFunc(m.Text, func(tok string) string {
			n, _ := strconv.Atoi(imageTokenRe.FindStringSubmatch(tok)[1])
			if gone[n] {
				return "[image omitted]"
			}
			return tok
		})
		m.Images = append([]llm.Image(nil), m.Images[drop:]...)
		left = 0
	}
}

// MaxRequestImages is a safety valve for provider limits on images per request,
// not a sliding window: dropping an old image rewrites the history and breaks
// the prompt cache from that point on, so images stay in the history until it is
// compacted (Compact moves old turns, images included, out of the requests).
const MaxRequestImages = 50
