package termimg

import (
	"image"
	"image/draw"
	_ "image/jpeg" // decoders for stored attachments
	_ "image/png"
)

// Resize scales src to w x h pixels by averaging the source pixels under every
// target pixel (alpha-weighted). It never crops. A target not smaller than the
// source returns the source converted to NRGBA unchanged: no upscaling.
func Resize(src image.Image, w, h int) *image.NRGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	in := image.NewNRGBA(image.Rect(0, 0, sw, sh))
	draw.Draw(in, in.Bounds(), src, b.Min, draw.Src)
	if w >= sw && h >= sh || w < 1 || h < 1 {
		return in
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0, y1 := y*sh/h, max(y*sh/h+1, (y+1)*sh/h)
		for x := 0; x < w; x++ {
			x0, x1 := x*sw/w, max(x*sw/w+1, (x+1)*sw/w)
			var r, g, bl, a, n int
			for yy := y0; yy < y1; yy++ {
				row := in.Pix[yy*in.Stride:]
				for xx := x0; xx < x1; xx++ {
					p := row[xx*4 : xx*4+4]
					al := int(p[3])
					r += int(p[0]) * al
					g += int(p[1]) * al
					bl += int(p[2]) * al
					a += al
					n++
				}
			}
			o := out.Pix[y*out.Stride+x*4:]
			if a > 0 {
				o[0], o[1], o[2] = uint8(r/a), uint8(g/a), uint8(bl/a)
			}
			o[3] = uint8(a / n)
		}
	}
	return out
}

// Box is the size in cells an image of w x h pixels takes when it must fit in
// maxCols x maxRows cells of cellW x cellH pixels each: scaled proportionally,
// never cropped, never enlarged. Both results are at least 1.
func Box(w, h, cellW, cellH, maxCols, maxRows int) (cols, rows int) {
	if w < 1 || h < 1 || cellW < 1 || cellH < 1 {
		return 1, 1
	}
	maxCols, maxRows = max(1, maxCols), max(1, maxRows)
	// Natural size in cells, then the largest scale (<= 1) that fits the box.
	natW, natH := float64(w)/float64(cellW), float64(h)/float64(cellH)
	scale := 1.0
	if natW > float64(maxCols) {
		scale = float64(maxCols) / natW
	}
	if natH*scale > float64(maxRows) {
		scale = float64(maxRows) / natH
	}
	cols = max(1, int(natW*scale+0.5))
	rows = max(1, int(natH*scale+0.5))
	return min(cols, maxCols), min(rows, maxRows)
}
