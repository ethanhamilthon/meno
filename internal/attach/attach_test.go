package attach

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPrepareKeepsSmallImageAsIs(t *testing.T) {
	data := pngBytes(t, 40, 30)
	img, err := Prepare(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img.Data, data) || img.Mime != MimePNG || img.W != 40 || img.H != 30 || len(img.SHA) != 64 {
		t.Fatalf("%+v", img)
	}
}

func TestPrepareScalesDownLargeImages(t *testing.T) {
	img, err := Prepare(pngBytes(t, 3136, 1000))
	if err != nil {
		t.Fatal(err)
	}
	if img.W != MaxSide || img.H != 500 {
		t.Fatalf("size %dx%d", img.W, img.H)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil || cfg.Width != img.W || cfg.Height != img.H {
		t.Fatalf("encoded image %v %v", cfg, err)
	}
	tall, err := Prepare(pngBytes(t, 700, 2000))
	if err != nil || tall.H != MaxSide || tall.W != 548 {
		t.Fatalf("tall: %+v %v", tall.W, err)
	}
}

func TestPrepareJPEGStaysJPEG(t *testing.T) {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2000, 10)), nil); err != nil {
		t.Fatal(err)
	}
	img, err := Prepare(b.Bytes())
	if err != nil || img.Mime != MimeJPEG || img.W != MaxSide {
		t.Fatalf("%+v %v", img.Mime, err)
	}
}

func TestPrepareRejects(t *testing.T) {
	var g bytes.Buffer
	_ = gif.Encode(&g, image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White}), nil)
	if _, err := Prepare(g.Bytes()); err == nil || !strings.Contains(err.Error(), "png and jpeg") {
		t.Fatalf("gif: %v", err)
	}
	if _, err := Prepare([]byte("hello")); err == nil {
		t.Fatal("text accepted")
	}
	if _, err := Prepare(make([]byte, MaxBytes+1)); err == nil || !strings.Contains(err.Error(), "5 MB") {
		t.Fatalf("big: %v", err)
	}
}

func TestStorePutIsIdempotent(t *testing.T) {
	st := Store{Dir: filepath.Join(t.TempDir(), "attachments")}
	img, _ := Prepare(pngBytes(t, 4, 4))
	for i := 0; i < 2; i++ {
		if err := st.Put(img); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(st.Path(img.SHA, img.Mime))
	if err != nil || !bytes.Equal(got, img.Data) {
		t.Fatalf("stored: %v", err)
	}
	entries, _ := os.ReadDir(st.Dir)
	if len(entries) != 1 {
		t.Fatalf("%d files", len(entries))
	}
}

func TestPathFromText(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "shot one.png")
	if err := os.WriteFile(plain, pngBytes(t, 2, 2), 0o644); err != nil {
		t.Fatal(err)
	}
	txt := filepath.Join(dir, "notes.txt")
	_ = os.WriteFile(txt, []byte("x"), 0o644)
	for name, in := range map[string]string{
		"plain":   plain,
		"escaped": strings.ReplaceAll(plain, " ", "\\ "),
		"quoted":  "'" + plain + "'",
		"url":     "file://" + strings.ReplaceAll(plain, " ", "%20"),
		"newline": plain + "\n",
	} {
		if got, ok := PathFromText(in, ""); !ok || got != plain {
			t.Errorf("%s: %q %v", name, got, ok)
		}
	}
	for name, in := range map[string]string{"text file": txt, "missing": filepath.Join(dir, "no.png"), "words": "hello world.png", "multi": plain + "\n" + plain, "relative": "a.png"} {
		if _, ok := PathFromText(in, ""); ok {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestClipboardLinuxPicksPNG(t *testing.T) {
	oldRun, oldLook := run, lookPath
	defer func() { run, lookPath = oldRun, oldLook }()
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	lookPath = func(string) (string, error) { return "/bin/x", nil }
	var calls []string
	run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if args[0] == "--list-types" {
			return []byte("text/plain\nimage/png\n"), nil
		}
		return []byte("PNGDATA"), nil
	}
	data, err := Clipboard("linux")
	if err != nil || string(data) != "PNGDATA" || calls[1] != "wl-paste --type image/png" {
		t.Fatalf("%q %v %v", data, err, calls)
	}
	run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("text/plain\n"), nil
	}
	if _, err := Clipboard("linux"); err != ErrNoImage {
		t.Fatalf("no image: %v", err)
	}
	if _, err := Clipboard("windows"); err == nil {
		t.Fatal("windows should be unsupported")
	}
}
