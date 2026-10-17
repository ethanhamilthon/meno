package jsvm

import (
	"errors"
	"fmt"
	"os"

	"meno/internal/attach"
)

// Viewer receives the images the code asks to look at. The image reaches the
// model with its next request, as a message after the result of this call.
// It returns the number the image gets among the images queued for that
// message; display is the path as the code wrote it.
type Viewer interface {
	View(display string, img attach.Image) (int, error)
}

// SetViewer enables view().
func (v *VM) SetViewer(x Viewer) { v.viewer.Store(&x) }

func (v *VM) viewerOrNil() Viewer {
	if p := v.viewer.Load(); p != nil {
		return *p
	}
	return nil
}

// view implements view(path): read a PNG or JPEG file and queue it for the model.
func (v *VM) view(p string) (string, error) {
	viewer := v.viewerOrNil()
	if viewer == nil {
		return "", errors.New("view: not available")
	}
	full, err := FS{Root: v.Root()}.path(p)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("view: %s is not a file", p)
	}
	if st.Size() > attach.MaxBytes {
		return "", fmt.Errorf("view: %s is larger than %d MB", p, attach.MaxBytes>>20)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	img, err := attach.Prepare(data)
	if err != nil {
		return "", fmt.Errorf("view: %s: %w", p, err)
	}
	n, err := viewer.View(p, img)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("queued image #%d: you will see it in the next message", n), nil
}
