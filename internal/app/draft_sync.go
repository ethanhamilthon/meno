package app

import (
	"path/filepath"
	"strconv"
	"strings"
)

// drafts is the store under the meno home; draftSaved is the last written
// state, so an unchanged composer causes no disk write.
func (a *App) drafts() draftStore { return draftStore{dir: filepath.Join(a.paths.Home, "drafts")} }

// currentDraft is the composer of the current session as a file.
func (a *App) currentDraft() (draftFile, map[string][]byte) {
	f := draftFile{Client: a.instance, Session: a.st.Runtime.ID(), Text: externalText(a.st.Input), Cursor: a.st.Cursor}
	data := map[string][]byte{}
	for _, im := range a.st.Images {
		f.Images = append(f.Images, im.attachment)
		if im.Data != nil {
			data[im.SHA] = im.Data
		}
	}
	return f, data
}

// saveDraft writes the composer when it changed since the last write. A failed
// write keeps the draft in memory and is reported once, never silently.
func (a *App) saveDraft() {
	if a.st.Runtime == nil || a.instance == "" {
		return
	}
	f, data := a.currentDraft()
	key := f.Session + "\x00" + f.Text + "\x00" + strconv.Itoa(len(f.Images))
	if key == a.draftSaved {
		return
	}
	if err := a.drafts().Save(f, data); err != nil {
		if !a.draftFailed {
			a.draftFailed = true
			a.notifyErr("draft not saved: " + err.Error())
		}
		return
	}
	a.draftFailed = false
	a.draftSaved = key
}

// announceDrafts tells the user about drafts left by other windows or a
// crashed run. They are never loaded into this session on their own.
func (a *App) announceDrafts() {
	if n := len(a.drafts().Recoverable(a.instance)); n > 0 {
		a.notify(strconv.Itoa(n) + " unsent draft(s) from an earlier window · /drafts to restore")
	}
}

// restoreDrafts moves the recoverable drafts into the composer, each one
// separated from the text already there; the stored files are then removed.
func (a *App) restoreDrafts() {
	st := a.drafts()
	list := st.Recoverable(a.instance)
	if len(list) == 0 {
		a.notify("no drafts to restore")
		return
	}
	var texts []string
	if cur := a.messageText(); strings.TrimSpace(cur) != "" {
		texts = append(texts, cur)
	}
	for _, f := range list {
		texts = append(texts, f.Text)
		_ = st.Remove(f.Client, f.Session)
	}
	a.setInput(a.runesFromText(strings.Join(texts, "\n\n")))
	a.notify("restored " + strconv.Itoa(len(list)) + " draft(s)")
}
