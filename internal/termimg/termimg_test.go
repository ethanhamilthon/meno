package termimg

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"strings"
	"testing"
)

func solid(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestKittyChunksAreWellFormedAndDecodeBack(t *testing.T) {
	noise := image.NewRGBA(image.Rect(0, 0, 120, 120))
	rng := rand.New(rand.NewSource(1))
	for i := range noise.Pix {
		noise.Pix[i] = byte(rng.Intn(256))
	}
	out, err := Place(Kitty, noise, 10, 5, 7)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	parts := strings.Split(strings.TrimSuffix(s, "\x1b\\"), "\x1b\\")
	if len(parts) < 2 {
		t.Fatalf("a large image must be chunked: %d", len(parts))
	}
	var payload strings.Builder
	for i, p := range parts {
		p = strings.TrimPrefix(p, "\x1b_G")
		head, data, ok := strings.Cut(p, ";")
		if !ok || len(data) > 4096 {
			t.Fatalf("chunk %d malformed or oversized", i)
		}
		last := i == len(parts)-1
		if (strings.Contains(head, "m=1") && last) || (strings.Contains(head, "m=0") && !last) {
			t.Fatalf("chunk %d has the wrong m flag: %s", i, head)
		}
		if i == 0 && !(strings.Contains(head, "a=T") && strings.Contains(head, "f=100") && strings.Contains(head, "i=7") && strings.Contains(head, "c=10") && strings.Contains(head, "r=5") && strings.Contains(head, "q=2")) {
			t.Fatalf("first chunk header: %s", head)
		}
		payload.WriteString(data)
	}
	raw, err := base64.StdEncoding.DecodeString(payload.String())
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(raw))
	if err != nil || got.Bounds().Dx() != 120 {
		t.Fatalf("payload is not the PNG: %v", err)
	}
	if string(Clear(Kitty)) != "\x1b_Ga=d,d=A,q=2\x1b\\" || Clear(Sixel) != nil {
		t.Fatal("clear sequences")
	}
}

func TestITermAndSixelShapes(t *testing.T) {
	out, _ := Place(ITerm, solid(4, 4, color.RGBA{255, 0, 0, 255}), 8, 2, 0)
	s := string(out)
	if !strings.HasPrefix(s, "\x1b]1337;File=inline=1;") || !strings.Contains(s, "width=8;height=2;preserveAspectRatio=1:") || !strings.HasSuffix(s, "\a") {
		t.Fatalf("iterm: %q", s[:60])
	}
	sx, _ := Place(Sixel, solid(12, 13, color.RGBA{255, 0, 0, 255}), 0, 0, 0)
	x := string(sx)
	if !strings.HasPrefix(x, "\x1bPq\"1;1;12;13") || !strings.HasSuffix(x, "\x1b\\") || strings.Count(x, "-") < 3 || !strings.Contains(x, "#180;2;100;0;0") {
		t.Fatalf("sixel: %q", x[:80])
	}
	// Fully transparent pixels paint nothing.
	tr, _ := Place(Sixel, solid(6, 6, color.RGBA{0, 0, 0, 0}), 0, 0, 0)
	if strings.Contains(string(tr)[strings.Index(string(tr), "-")-5:], "#") && strings.Count(string(tr), "#") > 216 {
		t.Fatal("transparent pixels must not be drawn")
	}
	if _, err := Place("bogus", solid(1, 1, color.White), 1, 1, 1); err == nil {
		t.Fatal("unknown protocol")
	}
}

func TestDetectIsConservative(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	for name, tc := range map[string]struct {
		env  map[string]string
		want string
	}{
		"kitty":       {map[string]string{"TERM": "xterm-kitty"}, Kitty},
		"ghostty":     {map[string]string{"TERM_PROGRAM": "ghostty"}, Kitty},
		"iterm":       {map[string]string{"TERM_PROGRAM": "iTerm.app"}, ITerm},
		"foot":        {map[string]string{"TERM": "foot"}, Sixel},
		"tmux":        {map[string]string{"TERM": "xterm-kitty", "TMUX": "/tmp/tmux"}, Off},
		"screen":      {map[string]string{"TERM": "screen-256color"}, Off},
		"plain xterm": {map[string]string{"TERM": "xterm-256color"}, Off},
		"nothing":     {map[string]string{}, Off},
	} {
		if got := Detect(env(tc.env)); got != tc.want {
			t.Errorf("%s: %s, want %s", name, got, tc.want)
		}
	}
	if Resolve("kitty", env(map[string]string{"TMUX": "x"})) != Kitty || Resolve("sixel", nil) != Sixel || Resolve("nonsense", nil) != Off {
		t.Fatal("a manual setting is taken as is, nonsense is off")
	}
}
