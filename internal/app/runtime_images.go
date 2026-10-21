package app

import "meno/internal/attach"

// insertImage adds a prepared image to the draft and puts its token in the
// input.
func (a *App) insertImage(img attach.Image, after bool) { a.insertImageFor(a.st.Runtime, img, after) }

func (a *App) insertImageFor(rt *Runtime, img attach.Image, after bool) {
	if rt.Ask != nil {
		a.imageError(rt, "can't attach an image while the agent asks a question")
		return
	}
	a.pruneImagesFor(rt)
	if len(rt.Images) >= attach.MaxImages {
		a.imageError(rt, "at most "+itoa(attach.MaxImages)+" images per message")
		return
	}
	if !a.imagesAllowedFor(rt) {
		return
	}
	n := 1
	for _, im := range rt.Images {
		n = max(n, im.N+1)
	}
	rt.Images = append(rt.Images, draftImage{
		attachment: attachment{N: n, Mime: img.Mime, SHA: img.SHA, W: img.W, H: img.H},
		Data:       img.Data,
	})
	tok := []rune("[Image #" + itoa(n) + "]")
	rt.clampCursor()
	a.insertAtCursorFor(rt, append(tok, ' '))
}

func (a *App) imageError(rt *Runtime, text string) {
	a.noticeFor(rt, text, true, false)
}

func (a *App) imagesAllowedFor(rt *Runtime) bool {
	p, m, _ := a.selectionFor(rt)
	for _, provider := range a.st.Providers {
		if provider.Name != p || !provider.Active() {
			continue
		}
		if info, ok := provider.Lookup(a.st.Pricing, m); ok && info.Vision != nil && !*info.Vision {
			a.imageError(rt, "model "+m+" does not accept images")
			return false
		}
	}
	return true
}
