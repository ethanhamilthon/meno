package attach

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNoImage means the clipboard holds no image.
var ErrNoImage = errors.New("no image in the clipboard")

// run executes a command and returns its output; tests replace it.
var run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

var lookPath = exec.LookPath

// Clipboard returns the image on the system clipboard (macOS: osascript;
// Linux: wl-paste or xclip).
func Clipboard(goos string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	switch goos {
	case "darwin":
		return clipboardDarwin(ctx)
	case "linux":
		return clipboardLinux(ctx)
	}
	return nil, errors.New("clipboard images are not supported on " + goos + " · use :attach <path>")
}

func clipboardDarwin(ctx context.Context) ([]byte, error) {
	dir, err := os.MkdirTemp("", "meno-clip-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "clip")
	for _, class := range []string{"PNGf", "JPEG"} {
		_, err := run(ctx, "osascript",
			"-e", "on run argv",
			"-e", "set d to (the clipboard as «class "+class+"»)",
			"-e", "set f to open for access (POSIX file (item 1 of argv)) with write permission",
			"-e", "set eof f to 0",
			"-e", "write d to f",
			"-e", "close access f",
			"-e", "end run", path)
		if err != nil {
			continue
		}
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return data, nil
		}
	}
	return nil, ErrNoImage
}

func clipboardLinux(ctx context.Context) ([]byte, error) {
	var list func() ([]byte, error)
	var get func(typ string) ([]byte, error)
	switch {
	case os.Getenv("WAYLAND_DISPLAY") != "" && have("wl-paste"):
		list = func() ([]byte, error) { return run(ctx, "wl-paste", "--list-types") }
		get = func(t string) ([]byte, error) { return run(ctx, "wl-paste", "--type", t) }
	case have("xclip"):
		list = func() ([]byte, error) { return run(ctx, "xclip", "-selection", "clipboard", "-t", "TARGETS", "-o") }
		get = func(t string) ([]byte, error) { return run(ctx, "xclip", "-selection", "clipboard", "-t", t, "-o") }
	default:
		return nil, errors.New("clipboard images need wl-paste or xclip · use :attach <path>")
	}
	types, err := list()
	if err != nil {
		return nil, ErrNoImage
	}
	for _, want := range []string{MimePNG, MimeJPEG} {
		for _, t := range strings.Fields(string(types)) {
			if t == want {
				if data, err := get(want); err == nil && len(data) > 0 {
					return data, nil
				}
			}
		}
	}
	return nil, ErrNoImage
}

func have(name string) bool {
	_, err := lookPath(name)
	return err == nil
}

// PathFromText reports whether text is just the path of an existing PNG or
// JPEG file, as a terminal pastes it when a file is dropped on it: possibly
// quoted, escaped with backslashes or written as a file:// URL.
func PathFromText(text, home string) (string, bool) {
	s := strings.TrimSpace(text)
	if s == "" || strings.Contains(s, "\n") {
		return "", false
	}
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	} else if strings.HasPrefix(s, "file://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", false
		}
		s = u.Path
	} else {
		s = unescape(s)
	}
	if strings.HasPrefix(s, "~/") && home != "" {
		s = filepath.Join(home, s[2:])
	}
	switch strings.ToLower(filepath.Ext(s)) {
	case ".png", ".jpg", ".jpeg":
	default:
		return "", false
	}
	if !filepath.IsAbs(s) {
		return "", false
	}
	if st, err := os.Stat(s); err != nil || !st.Mode().IsRegular() {
		return "", false
	}
	return s, true
}

// unescape removes the backslashes a shell-style drop puts before spaces and
// other special characters.
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
