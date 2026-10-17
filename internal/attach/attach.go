// Package attach prepares images for the conversation: it checks and shrinks
// them, keeps the bytes on disk and reads images from the system clipboard.
package attach

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

const (
	MaxImages = 5       // images per message
	MaxBytes  = 5 << 20 // size of one image before and after preparation
	MaxSide   = 1568    // longer side in pixels; bigger images are scaled down
)

const (
	MimePNG  = "image/png"
	MimeJPEG = "image/jpeg"
)

// Image is a prepared image: PNG or JPEG bytes that fit the limits.
type Image struct {
	Data []byte
	Mime string
	W, H int
	SHA  string
}

// Ext is the file extension for a mime type.
func Ext(mime string) string {
	if mime == MimeJPEG {
		return ".jpg"
	}
	return ".png"
}

// Prepare checks that data is a PNG or JPEG within the limits and scales it
// down when its longer side exceeds MaxSide.
func Prepare(data []byte) (Image, error) {
	if len(data) > MaxBytes {
		return Image{}, fmt.Errorf("image is larger than %d MB", MaxBytes>>20)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Image{}, errors.New("not an image (png and jpeg are supported)")
	}
	var mime string
	switch format {
	case "png":
		mime = MimePNG
	case "jpeg":
		mime = MimeJPEG
	default:
		return Image{}, fmt.Errorf("unsupported image format %s (png and jpeg only)", format)
	}
	img := Image{Data: data, Mime: mime, W: cfg.Width, H: cfg.Height}
	if max(cfg.Width, cfg.Height) > MaxSide {
		src, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return Image{}, fmt.Errorf("decode image: %w", err)
		}
		dst := scale(src, MaxSide)
		var buf bytes.Buffer
		if mime == MimePNG {
			err = png.Encode(&buf, dst)
		} else {
			err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85})
		}
		if err != nil {
			return Image{}, fmt.Errorf("encode image: %w", err)
		}
		if buf.Len() > MaxBytes {
			return Image{}, fmt.Errorf("image is larger than %d MB after scaling", MaxBytes>>20)
		}
		img.Data, img.W, img.H = buf.Bytes(), dst.Bounds().Dx(), dst.Bounds().Dy()
	}
	sum := sha256.Sum256(img.Data)
	img.SHA = hex.EncodeToString(sum[:])
	return img, nil
}

// scale shrinks src so that its longer side is side pixels, averaging the
// source pixels under every target pixel.
func scale(src image.Image, side int) *image.NRGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dw, dh := side, max(1, sh*side/sw)
	if sh > sw {
		dw, dh = max(1, sw*side/sh), side
	}
	in := image.NewNRGBA(image.Rect(0, 0, sw, sh))
	draw.Draw(in, in.Bounds(), src, b.Min, draw.Src)
	out := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		y0, y1 := y*sh/dh, max(y*sh/dh+1, (y+1)*sh/dh)
		for x := 0; x < dw; x++ {
			x0, x1 := x*sw/dw, max(x*sw/dw+1, (x+1)*sw/dw)
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

// Store keeps prepared images as files named after their hash.
type Store struct{ Dir string }

func (s Store) Path(sha, mime string) string { return filepath.Join(s.Dir, sha+Ext(mime)) }

// Put writes the image unless a file with its hash is already there.
func (s Store) Put(img Image) error {
	path := s.Path(img.SHA, img.Mime)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Dir, ".tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(img.Data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
