package app

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sys/unix"

	"meno/internal/attach"
	"meno/internal/safe"
	"meno/internal/termimg"
	"meno/internal/ui"
)

// imageMgr draws inline pictures with a terminal graphics protocol. The
// timeline reserves blank rows for every image; after each frame the manager
// paints the pictures that are fully visible over those rows, and removes them
// again when the set changes (scroll, resize, dialogs, session switch). All
// writes happen on the UI goroutine through the terminal tcell owns.
type imageMgr struct {
	mu     sync.Mutex
	cache  map[string]*decodedImage
	order  []string
	placed string // description of what is on screen now
	out    io.Writer
	vis    []visImage
	proto  string // protocol resolved for this frame
}

type decodedImage struct {
	img   image.Image
	err   error
	ready bool
}

type visImage struct {
	ref  *ui.ImageRef
	x, y int
}

type imageDecoded struct{}

const imageCacheMax = 32

// cellSize is the pixel size of a terminal cell from the window size, or the
// common 8x16 when the terminal does not report pixels.
func cellSize() (w, h int) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Xpixel == 0 || ws.Ypixel == 0 || ws.Col == 0 || ws.Row == 0 {
		return 8, 16
	}
	return max(1, int(ws.Xpixel)/int(ws.Col)), max(1, int(ws.Ypixel)/int(ws.Row))
}

// writer is where graphics bytes go: the terminal behind tcell, or a test sink.
func (a *App) imageWriter() io.Writer {
	if a.imgs.out != nil {
		return a.imgs.out
	}
	if tty, ok := a.scr.Tty(); ok {
		return tty
	}
	return nil
}

// imageProto is the protocol in use for this frame ("off" without a terminal
// to write to).
func (a *App) imageProto() string {
	if a.imageWriter() == nil {
		return termimg.Off
	}
	return termimg.Resolve(a.st.Settings.ImagesProtocol, os.Getenv)
}

// imageOpts are the layout options for inline images.
func (a *App) imageOpts() ui.ImageOpts {
	if a.imageProto() == termimg.Off {
		return ui.ImageOpts{}
	}
	w, h := cellSize()
	_, sh := a.scr.Size()
	return ui.ImageOpts{Enabled: true, CellW: w, CellH: h, MaxRows: max(3, (sh-a.bottomHeight())/2)}
}

// noteVisible records the images of a drawn log window that are fully inside
// it; a partly visible picture is hidden and keeps only its caption.
func (a *App) noteVisible(lines []ui.Line, start, x, y, h int) {
	a.imgs.vis = a.imgs.vis[:0]
	if a.st.Over != OverNone || a.st.Ask != nil {
		return
	}
	for i := 0; i < h && start+i < len(lines); i++ {
		ref := lines[start+i].Image
		if ref == nil || i+ref.Rows > h {
			continue
		}
		a.imgs.vis = append(a.imgs.vis, visImage{ref, x + ui.PadX + 2, y + i})
	}
}

func (m *imageMgr) key() string {
	parts := make([]string, 0, len(m.vis))
	for _, v := range m.vis {
		parts = append(parts, fmt.Sprintf("%s@%d,%d/%dx%d", v.ref.SHA, v.x, v.y, v.ref.Cols, v.ref.Rows))
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

// get returns the scaled picture, starting a background decode on a miss.
func (a *App) imageGet(ref *ui.ImageRef, cw, ch int) (image.Image, bool) {
	key := fmt.Sprintf("%s|%d|%d", ref.SHA, ref.Cols, ref.Rows)
	m := &a.imgs
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cache == nil {
		m.cache = map[string]*decodedImage{}
	}
	if e, ok := m.cache[key]; ok {
		return e.img, e.ready && e.err == nil
	}
	e := &decodedImage{}
	m.cache[key] = e
	m.order = append(m.order, key)
	if len(m.order) > imageCacheMax {
		delete(m.cache, m.order[0])
		m.order = m.order[1:]
	}
	path := filepath.Join(a.attachDir(), ref.SHA+attach.Ext(ref.Mime))
	cols, rows := ref.Cols, ref.Rows
	a.running.Add(1)
	go func() {
		defer safe.Recover(a.reportPanic)
		defer a.running.Done()
		img, err := loadScaled(path, cols*cw, rows*ch)
		m.mu.Lock()
		e.img, e.err, e.ready = img, err, true
		m.mu.Unlock()
		a.postEvent(imageDecoded{})
	}()
	return nil, false
}

func loadScaled(path string, maxW, maxH int) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	tw := min(w, maxW)
	th := max(1, h*tw/max(1, w))
	if th > maxH {
		th = maxH
		tw = max(1, w*th/max(1, h))
	}
	return termimg.Resize(img, tw, th), nil
}

// placeImages paints (or removes) the pictures after a frame was shown.
func (a *App) placeImages() {
	m := &a.imgs
	proto := a.imageProto()
	out := a.imageWriter()
	if out == nil || proto == termimg.Off {
		if m.placed != "" && out != nil {
			_, _ = out.Write(termimg.Clear(m.proto))
		}
		m.placed, m.proto = "", proto
		return
	}
	key := m.key()
	if key == m.placed && proto == m.proto {
		return
	}
	cw, ch := cellSize()
	var buf bytes.Buffer
	complete := true
	for _, v := range m.vis {
		img, ok := a.imageGet(v.ref, cw, ch)
		if !ok {
			complete = false
			continue
		}
		data, err := termimg.Place(proto, img, v.ref.Cols, v.ref.Rows, uint32(1+len(buf.Bytes())%1000))
		if err != nil {
			continue
		}
		fmt.Fprintf(&buf, "\x1b7\x1b[%d;%dH", v.y+1, v.x+1)
		buf.Write(data)
		buf.WriteString("\x1b8")
	}
	if m.placed != "" {
		if proto == termimg.Kitty || m.proto == termimg.Kitty {
			_, _ = out.Write(termimg.Clear(termimg.Kitty))
		}
		if proto != termimg.Kitty {
			a.scr.Sync() // pixels painted over cells are only removed by repainting them
		}
	}
	_, _ = out.Write(buf.Bytes())
	m.proto = proto
	if complete {
		m.placed = key
	} else {
		m.placed = "" // some picture is still decoding: try again when it is ready
		if buf.Len() == 0 {
			m.placed = "pending"
		}
	}
}

// clearImages removes every picture before meno hands the terminal back.
func (a *App) clearImages() {
	if out := a.imageWriter(); out != nil && a.imgs.placed != "" {
		_, _ = out.Write(termimg.Clear(a.imgs.proto))
		a.imgs.placed = ""
	}
}
