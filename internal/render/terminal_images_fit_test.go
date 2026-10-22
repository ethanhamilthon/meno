package render

import (
	"image"
	"image/color"
	"testing"
)

func TestBoxScalesProportionallyNeverCropsNorEnlarges(t *testing.T) {
	// 8x16 pixel cells. A 160x160 image is 20x10 cells naturally.
	cases := []struct{ w, h, maxC, maxR, wc, wr int }{
		{160, 160, 100, 100, 20, 10}, // fits: natural size, no upscaling
		{160, 160, 10, 100, 10, 5},   // width limited
		{160, 160, 100, 4, 8, 4},     // height limited
		{1600, 160, 40, 40, 40, 2},   // wide strip
		{8, 16, 80, 20, 1, 1},        // tiny image: one cell
		{800, 3200, 80, 10, 5, 10},   // tall image limited by rows
	}
	for _, c := range cases {
		cols, rows := Box(c.w, c.h, 8, 16, c.maxC, c.maxR)
		if cols != c.wc || rows != c.wr {
			t.Errorf("%dx%d in %dx%d => %dx%d, want %dx%d", c.w, c.h, c.maxC, c.maxR, cols, rows, c.wc, c.wr)
		}
		if cols > c.maxC || rows > c.maxR {
			t.Errorf("exceeds the box: %dx%d", cols, rows)
		}
	}
	if c, r := Box(0, 0, 8, 16, 10, 10); c != 1 || r != 1 {
		t.Fatal("degenerate input")
	}
}

func TestResizeAveragesAndKeepsAlpha(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if x < 2 {
				src.Set(x, y, color.NRGBA{255, 0, 0, 255})
			} else {
				src.Set(x, y, color.NRGBA{0, 0, 0, 0}) // transparent half
			}
		}
	}
	out := Resize(src, 2, 2)
	if out.Bounds().Dx() != 2 || out.NRGBAAt(0, 0) != (color.NRGBA{255, 0, 0, 255}) || out.NRGBAAt(1, 0).A != 0 {
		t.Fatalf("%v %v", out.NRGBAAt(0, 0), out.NRGBAAt(1, 0))
	}
	if same := Resize(src, 8, 8); same.Bounds().Dx() != 4 {
		t.Fatal("no upscaling")
	}
}
