// Package render encodes images for terminal graphics protocols. It only
// builds byte sequences; where they are written and when is the application's
// business (internal/tui), so nothing here touches the terminal.
package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strconv"
	"strings"
)

// Protocol names. Off and the zero value mean no graphics.
const (
	Off    = "off"
	Kitty  = "kitty"
	ITerm  = "iterm"
	Sixel  = "sixel"
	Auto   = "auto"
	chunk  = 4096
	escAPC = "\x1b_G"
	st     = "\x1b\\"
)

// Valid reports whether name is an images.protocol setting value.
func Valid(name string) bool {
	switch name {
	case Auto, Off, Kitty, ITerm, Sixel:
		return true
	}
	return false
}

// Place returns the bytes that draw img at the current cursor position,
// occupying cols x rows cells, in the given protocol. id (> 0) names the image
// so Kitty can address it later.
func Place(proto string, img image.Image, cols, rows int, id uint32) ([]byte, error) {
	switch proto {
	case Kitty:
		return kitty(img, cols, rows, id)
	case ITerm:
		return iterm(img, cols, rows)
	case Sixel:
		return sixel(img), nil
	}
	return nil, fmt.Errorf("termimg: unknown protocol %q", proto)
}

// Clear returns the bytes that remove every image this package placed through
// the protocol (Kitty keeps images until told otherwise; the others are
// painted over by the application).
func Clear(proto string) []byte {
	if proto == Kitty {
		return []byte(escAPC + "a=d,d=A,q=2" + st)
	}
	return nil
}

func kitty(img image.Image, cols, rows int, id uint32) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	data := base64.StdEncoding.EncodeToString(buf.Bytes())
	var out strings.Builder
	first := true
	for len(data) > 0 {
		n := min(chunk, len(data))
		more := 0
		if n < len(data) {
			more = 1
		}
		if first {
			// a=T transmit and display; f=100 PNG; c/r scale to the cell box;
			// C=1 leaves the cursor alone; q=2 silences the terminal's replies,
			// which would otherwise arrive as typed characters.
			out.WriteString(fmt.Sprintf("%sa=T,f=100,i=%d,c=%d,r=%d,C=1,q=2,m=%d;%s%s", escAPC, id, cols, rows, more, data[:n], st))
			first = false
		} else {
			out.WriteString(fmt.Sprintf("%sm=%d,q=2;%s%s", escAPC, more, data[:n], st))
		}
		data = data[n:]
	}
	return []byte(out.String()), nil
}

func iterm(img image.Image, cols, rows int) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return []byte(fmt.Sprintf("\x1b]1337;File=inline=1;size=%d;width=%d;height=%d;preserveAspectRatio=1:%s\a", buf.Len(), cols, rows, b64)), nil
}

// sixel encodes img with a fixed 6x6x6 colour cube (216 colours): simple,
// deterministic and good enough for a preview. Transparent pixels are left
// unpainted.
func sixel(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var out strings.Builder
	out.WriteString("\x1bPq")
	out.WriteString(fmt.Sprintf("\"1;1;%d;%d", w, h))
	for i := 0; i < 216; i++ {
		r, g, bl := i/36, (i/6)%6, i%6
		out.WriteString(fmt.Sprintf("#%d;2;%d;%d;%d", i, r*20, g*20, bl*20))
	}
	idx := func(x, y int) int {
		r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
		if a < 0x8000 {
			return -1
		}
		q := func(v uint32) int { return int(v>>8) * 5 / 255 }
		return q(r)*36 + q(g)*6 + q(bl)
	}
	for y0 := 0; y0 < h; y0 += 6 {
		colors := map[int][]byte{}
		for x := 0; x < w; x++ {
			for dy := 0; dy < 6 && y0+dy < h; dy++ {
				c := idx(x, y0+dy)
				if c < 0 {
					continue
				}
				if colors[c] == nil {
					colors[c] = make([]byte, w)
				}
				colors[c][x] |= 1 << dy
			}
		}
		first := true
		for c := 0; c < 216; c++ {
			bits, ok := colors[c]
			if !ok {
				continue
			}
			if !first {
				out.WriteByte('$')
			}
			first = false
			out.WriteString("#" + strconv.Itoa(c))
			out.WriteString(runLength(bits))
		}
		out.WriteByte('-')
	}
	out.WriteString("\x1b\\")
	return []byte(out.String())
}

// runLength writes sixel data bytes with !n repeats.
func runLength(bits []byte) string {
	var b strings.Builder
	for i := 0; i < len(bits); {
		j := i
		for j < len(bits) && bits[j] == bits[i] {
			j++
		}
		ch := byte(63 + bits[i])
		if n := j - i; n > 3 {
			b.WriteString("!" + strconv.Itoa(n) + string(ch))
		} else {
			b.WriteString(strings.Repeat(string(ch), n))
		}
		i = j
	}
	return b.String()
}
