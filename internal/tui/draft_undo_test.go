package tui

import (
	"testing"

	"meno/internal/content"
)

// Deleting an [Image #N] token and undoing must bring the image back, not only the text.
func TestUndoRestoresDeletedImage(t *testing.T) {
	a, _ := newTestApp(t)
	img := content.Image{Mime: "image/png", SHA: "abc", W: 1, H: 1, Data: []byte{1}}
	a.insertImage(img, false)
	if len(a.st.Images) != 1 {
		t.Fatalf("image not attached: %d", len(a.st.Images))
	}
	before := a.snapshot()
	a.setInput(nil)
	a.pruneImages()
	a.pushUndo(before)
	if len(a.st.Images) != 0 {
		t.Fatal("image should be pruned with its token")
	}
	a.undo()
	if len(a.st.Images) != 1 || a.st.Images[0].SHA != "abc" {
		t.Fatalf("images after undo: %+v", a.st.Images)
	}
}
