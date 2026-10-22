package tui

import "strings"

// setDraft replaces the composer text and its images.
func (a *App) setDraft(m msgDraft, data map[string][]byte) {
	a.setDraftFor(a.st.Runtime, m, data)
}

func (a *App) setDraftFor(rt *Runtime, m msgDraft, data map[string][]byte) {
	a.setInputFor(rt, a.runesFromText(m.Text))
	rt.Images = nil
	for _, at := range m.Attachments {
		rt.Images = append(rt.Images, draftImage{attachment: at, Data: data[at.SHA]})
	}
}

// restoreDraft puts queued messages back into the composer, after what is
// already there.
func (a *App) restoreDraft(rest []msgDraft) { a.restoreDraftFor(a.st.Runtime, rest) }

func (a *App) restoreDraftFor(rt *Runtime, rest []msgDraft) {
	if len(rest) == 0 {
		return
	}
	a.pruneImagesFor(rt)
	data := map[string][]byte{}
	for _, im := range rt.Images {
		data[im.SHA] = im.Data
	}
	for _, m := range rest {
		for k, v := range m.data {
			data[k] = v
		}
	}
	var texts []string
	var atts []attachment
	if cur := externalText(rt.Input); strings.TrimSpace(cur) != "" {
		texts = append(texts, cur)
		for _, im := range rt.Images {
			atts = append(atts, im.attachment)
		}
	}
	for _, m := range rest {
		texts = append(texts, m.Text)
		atts = append(atts, m.Attachments...)
	}
	a.setDraftFor(rt, msgDraft{Text: strings.Join(texts, "\n\n"), Attachments: atts}, data)
}
