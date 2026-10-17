package llm

import (
	"encoding/base64"
	"os"
	"regexp"
	"strconv"
)

var imageTokenRe = regexp.MustCompile(`\[Image #(\d+)\]`)

// contentPart is one piece of a user message: text or an image.
type contentPart struct {
	text    string
	dataURL string // set for an image
}

// messageParts splits the text of m at its image tokens and puts the images in
// place. An image whose file cannot be read becomes a note. Images that the
// text does not mention come last.
func messageParts(m Msg) []contentPart {
	byN := map[int]Image{}
	for _, im := range m.Images {
		byN[im.N] = im
	}
	used := map[int]bool{}
	var parts []contentPart
	add := func(im Image) {
		data, err := os.ReadFile(im.Path)
		if err != nil {
			parts = append(parts, contentPart{text: "[image unavailable]"})
			return
		}
		parts = append(parts, contentPart{dataURL: "data:" + im.Mime + ";base64," + base64.StdEncoding.EncodeToString(data)})
	}
	last := 0
	for _, loc := range imageTokenRe.FindAllStringSubmatchIndex(m.Text, -1) {
		n, _ := strconv.Atoi(m.Text[loc[2]:loc[3]])
		im, ok := byN[n]
		if !ok || used[n] {
			continue
		}
		used[n] = true
		parts = append(parts, contentPart{text: m.Text[last:loc[1]]})
		add(im)
		last = loc[1]
	}
	if last < len(m.Text) || len(parts) == 0 {
		parts = append(parts, contentPart{text: m.Text[last:]})
	}
	for _, im := range m.Images {
		if !used[im.N] {
			used[im.N] = true
			add(im)
		}
	}
	return parts
}

// userContent is the content of a user message: a plain string, or a list of
// text and image parts when the message has images. textType and imageType
// name the part kinds of the protocol; chat wraps the image url in an object.
func userContent(m Msg, textType, imageType string, wrap bool) any {
	if len(m.Images) == 0 {
		return m.Text
	}
	var out []map[string]any
	for _, p := range messageParts(m) {
		switch {
		case p.dataURL != "" && wrap:
			out = append(out, map[string]any{"type": imageType, "image_url": map[string]any{"url": p.dataURL}})
		case p.dataURL != "":
			out = append(out, map[string]any{"type": imageType, "image_url": p.dataURL})
		case p.text != "":
			out = append(out, map[string]any{"type": textType, "text": p.text})
		}
	}
	return out
}
