package ui

import (
	"fmt"
	"strings"

	"meno/internal/termimg"
)

// ImageRef marks the first reserved row of an inline image in the timeline.
// The terminal draws the picture over Rows rows of blank cells; SHA and Mime
// say which stored bytes to show.
type ImageRef struct {
	SHA, Mime  string
	N, W, H    int
	Cols, Rows int
}

// ImageOpts configure inline images. Disabled, every image is only a caption.
type ImageOpts struct {
	Enabled      bool
	CellW, CellH int // pixel size of a terminal cell
	MaxRows      int // half of the visible transcript height
}

// imageBlock is the caption of an attachment and, when graphics are on, the
// rows its picture covers. The caption always stays: it names the number,
// size and type, and it is all there is without graphics.
func imageBlock(at Attachment, source string, wrapW int, o ImageOpts) []Line {
	kind := strings.TrimPrefix(at.Mime, "image/")
	caption := fmt.Sprintf("Image #%d · %d×%d · %s · %s", at.N, at.W, at.H, kind, source)
	out := transcriptRows(caption, "▣", wrapW)
	if !o.Enabled || at.W < 1 || at.H < 1 {
		return out
	}
	cols, rows := termimg.Box(at.W, at.H, max(1, o.CellW), max(1, o.CellH), TranscriptTextWidth(wrapW), max(1, o.MaxRows))
	ref := &ImageRef{SHA: at.SHA, Mime: at.Mime, N: at.N, W: at.W, H: at.H, Cols: cols, Rows: rows}
	for i := 0; i < rows; i++ {
		l := Band(BgTerm)
		if i == 0 {
			l.Image = ref
		}
		out = append(out, l)
	}
	return out
}
