package render

import (
	"strings"
	"testing"
)

func TestImageBlocksReserveRowsOrFallBackToCaption(t *testing.T) {
	at := Attachment{N: 1, Mime: "image/png", SHA: "abc", W: 160, H: 160}
	turns := []Turn{
		{Role: RoleUser, Text: "see [Image #1]", Attachments: []Attachment{at}},
		{Role: RoleUser, Kind: KindView, Text: "Image from view(\"x.png\")", Attachments: []Attachment{{N: 1, Mime: "image/jpeg", SHA: "def", W: 800, H: 3200}}},
	}
	off := BuildLines(turns, 100, JSOpts{})
	for _, l := range off {
		if l.Image != nil {
			t.Fatal("no graphics: captions only")
		}
	}
	var captions []string
	for _, l := range off {
		if strings.Contains(l.Text(), "▣ Image") {
			captions = append(captions, strings.TrimSpace(l.Text()))
		}
	}
	if len(captions) != 2 || captions[0] != "▣ Image #1 · 160×160 · png · attachment" || captions[1] != "▣ Image #1 · 800×3200 · jpeg · view()" {
		t.Fatalf("captions: %q", captions)
	}
	on := BuildLines(turns, 100, JSOpts{Images: ImageOpts{Enabled: true, CellW: 8, CellH: 16, MaxRows: 12}})
	var refs []*ImageRef
	for _, l := range on {
		if l.Image != nil {
			refs = append(refs, l.Image)
		}
	}
	if len(refs) != 2 || refs[0].Cols != 20 || refs[0].Rows != 10 {
		t.Fatalf("a 160x160 image is 20x10 cells: %+v", refs)
	}
	if refs[1].Rows > 12 || refs[1].Rows < 1 || refs[1].Cols < 1 {
		t.Fatalf("tall image limited by half the transcript: %+v", refs[1])
	}
	// The reserved rows follow the caption and are blank.
	if len(on)-len(off) != refs[0].Rows+refs[1].Rows {
		t.Fatalf("layout grew by %d rows, want %d", len(on)-len(off), refs[0].Rows+refs[1].Rows)
	}
}

func TestImageBlocksFollowViewMode(t *testing.T) {
	for _, kind := range []string{"", KindView} {
		for _, view := range []ViewMode{ViewFull, ViewBashOnly, ViewCompact} {
			for _, enabled := range []bool{false, true} {
				turn := Turn{Role: RoleUser, Kind: kind, Text: "look [Image #1]", Attachments: []Attachment{{N: 1, Mime: "image/png", SHA: "abc", W: 160, H: 160}}}
				opts := JSOpts{View: view, Images: ImageOpts{Enabled: enabled, CellW: 8, CellH: 16, MaxRows: 12}}
				lines := BuildLines([]Turn{turn}, 100, opts)
				refs, captions := 0, 0
				for _, l := range lines {
					if l.Image != nil {
						refs++
					}
					if strings.Contains(l.Text(), "▣ Image") {
						captions++
					}
				}
				if !strings.Contains(summaryText(lines), turn.Text) {
					t.Fatal("ordinary message text was lost")
				}
				if view == ViewCompact {
					turn.Attachments = nil
					plain := BuildLines([]Turn{turn}, 100, opts)
					if refs != 0 || captions != 0 || len(lines) != len(plain) {
						t.Fatalf("compact kind=%q enabled=%v: refs=%d captions=%d rows=%d, want %d", kind, enabled, refs, captions, len(lines), len(plain))
					}
				} else if captions != 1 || enabled && refs != 1 || !enabled && refs != 0 {
					t.Fatalf("view=%d kind=%q enabled=%v: refs=%d captions=%d", view, kind, enabled, refs, captions)
				}
			}
		}
	}
}
