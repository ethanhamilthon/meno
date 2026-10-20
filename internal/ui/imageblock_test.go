package ui

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
